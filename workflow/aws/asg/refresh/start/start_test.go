// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package start

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_asg_refresh"

// describeResponse lists groups that exercise every branch of the launch
// template match: a direct launch template, a mixed instances policy, no
// template at all, a scaled-to-zero group, and a template that is not wanted.
const describeResponse = `<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups>
      <member>
        <AutoScalingGroupName>web-asg</AutoScalingGroupName>
        <MinSize>1</MinSize><MaxSize>3</MaxSize><DesiredCapacity>2</DesiredCapacity>
        <LaunchTemplate><LaunchTemplateId>lt-web</LaunchTemplateId><Version>$Default</Version></LaunchTemplate>
      </member>
      <member>
        <AutoScalingGroupName>mixed-asg</AutoScalingGroupName>
        <MinSize>0</MinSize><MaxSize>3</MaxSize><DesiredCapacity>1</DesiredCapacity>
        <MixedInstancesPolicy><LaunchTemplate><LaunchTemplateSpecification>
          <LaunchTemplateId>lt-mixed</LaunchTemplateId><Version>$Default</Version>
        </LaunchTemplateSpecification></LaunchTemplate></MixedInstancesPolicy>
      </member>
      <member>
        <AutoScalingGroupName>config-asg</AutoScalingGroupName>
        <MinSize>1</MinSize><MaxSize>1</MaxSize><DesiredCapacity>1</DesiredCapacity>
        <LaunchConfigurationName>legacy</LaunchConfigurationName>
      </member>
      <member>
        <AutoScalingGroupName>mixed-noid-asg</AutoScalingGroupName>
        <MinSize>1</MinSize><MaxSize>1</MaxSize><DesiredCapacity>1</DesiredCapacity>
        <MixedInstancesPolicy><LaunchTemplate><LaunchTemplateSpecification>
          <LaunchTemplateName>by-name</LaunchTemplateName>
        </LaunchTemplateSpecification></LaunchTemplate></MixedInstancesPolicy>
      </member>
      <member>
        <AutoScalingGroupName>idle-asg</AutoScalingGroupName>
        <MinSize>0</MinSize><MaxSize>3</MaxSize><DesiredCapacity>0</DesiredCapacity>
        <LaunchTemplate><LaunchTemplateId>lt-web</LaunchTemplateId><Version>$Default</Version></LaunchTemplate>
      </member>
      <member>
        <AutoScalingGroupName>other-asg</AutoScalingGroupName>
        <MinSize>1</MinSize><MaxSize>3</MaxSize><DesiredCapacity>1</DesiredCapacity>
        <LaunchTemplate><LaunchTemplateId>lt-other</LaunchTemplateId><Version>$Default</Version></LaunchTemplate>
      </member>
    </AutoScalingGroups>
  </DescribeAutoScalingGroupsResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</DescribeAutoScalingGroupsResponse>`

const refreshResponse = `<StartInstanceRefreshResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <StartInstanceRefreshResult><InstanceRefreshId>08b91cf7-8fa6-48af-b6a6-d227f40f1b9b</InstanceRefreshId></StartInstanceRefreshResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</StartInstanceRefreshResponse>`

type reply struct {
	status int
	body   string
}

func ok(body string) reply { return reply{http.StatusOK, body} }

func serve(t *testing.T, replies map[string]reply) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		if r, found := replies[action]; found {
			return r.status, r.body
		}
		return awstest.Unknown(action)
	})
}

func happy() map[string]reply {
	return map[string]reply{
		"DescribeAutoScalingGroups": ok(describeResponse),
		"StartInstanceRefresh":      ok(refreshResponse),
	}
}

func TestExecute_Refresh(t *testing.T) {
	rec := serve(t, happy())
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{
		"launch_templates": []string{"LT-WEB", "lt-mixed"},
		"filters":          []map[string]any{{"name": "tag:env", "values": []string{"prod"}}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS Autoscaling Groups refreshed", r.Msg)
	assert.Equal(t, 2, r.Data["asg_refresh_count"])
	assert.Equal(t, map[string]string{"web-asg": "success", "mixed-asg": "success"}, r.Data["asg_refresh_results"])

	describe := rec.Forms("DescribeAutoScalingGroups")
	require.Len(t, describe, 1)
	assert.Equal(t, "tag:env", describe[0].Get("Filters.member.1.Name"))
	assert.Equal(t, "10", describe[0].Get("MaxRecords"))

	refresh := rec.Forms("StartInstanceRefresh")
	require.Len(t, refresh, 2)
	assert.Equal(t, "web-asg", refresh[0].Get("AutoScalingGroupName"))
	assert.Equal(t, "mixed-asg", refresh[1].Get("AutoScalingGroupName"))
	f := refresh[0]
	assert.Equal(t, "true", f.Get("Preferences.SkipMatching"))
	assert.Equal(t, "false", f.Get("Preferences.AutoRollback"))
	assert.Equal(t, "Wait", f.Get("Preferences.ScaleInProtectedInstances"))
	assert.Equal(t, "Wait", f.Get("Preferences.StandbyInstances"))
	assert.Equal(t, "110", f.Get("Preferences.MaxHealthyPercentage"))
	assert.Equal(t, "100", f.Get("Preferences.MinHealthyPercentage"))
	assert.Equal(t, "300", f.Get("Preferences.InstanceWarmup"))
}

func TestExecute_SkipMatchingFalse(t *testing.T) {
	rec := serve(t, happy())
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{"launch_templates": []string{"lt-web"}, "skip_matching": "false"})
	require.True(t, r.Success, r.Msg)
	refresh := rec.Forms("StartInstanceRefresh")
	require.Len(t, refresh, 1)
	assert.Equal(t, "false", refresh[0].Get("Preferences.SkipMatching"))
}

func TestExecute_Select(t *testing.T) {
	rec := serve(t, happy())
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"launch_templates": []string{"lt-web", "lt-mixed"},
		"select":           []map[string]any{{"field": "MinSize", "value": 0, "compare": "equal"}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 1, r.Data["asg_refresh_count"])
	assert.Equal(t, map[string]string{"mixed-asg": "success"}, r.Data["asg_refresh_results"])
	assert.Equal(t, 1, rec.Count("StartInstanceRefresh"))
}

func TestExecute_DryRun(t *testing.T) {
	rec := serve(t, map[string]reply{"DescribeAutoScalingGroups": ok(describeResponse)})
	r := awstest.Run(t, taskID, shared.TaskContext{DryRun: true}, map[string]any{"launch_templates": []string{"lt-web"}})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "Dry run, ASG refresh simulated", r.Msg)
	assert.Equal(t, 1, r.Data["asg_refresh_count"])
	assert.Equal(t, map[string]string{"web-asg": "success"}, r.Data["asg_refresh_results"])
	assert.Equal(t, 0, rec.Count("StartInstanceRefresh"))
}

func TestExecute_RefreshFails(t *testing.T) {
	serve(t, map[string]reply{
		"DescribeAutoScalingGroups": ok(describeResponse),
		"StartInstanceRefresh":      {http.StatusBadRequest, awstest.ASGError("InstanceRefreshInProgress", "An Instance Refresh is already in progress")},
	})
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{"launch_templates": []string{"lt-web", "lt-mixed"}})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, 2, r.Data["asg_refresh_count"])
	results, isMap := r.Data["asg_refresh_results"].(map[string]string)
	require.True(t, isMap)
	for _, name := range []string{"web-asg", "mixed-asg"} {
		assert.Contains(t, results[name], "Instance Refresh failed: ")
		assert.Contains(t, results[name], "InstanceRefreshInProgress")
	}
}

func TestExecute_Errors(t *testing.T) {
	cases := []struct {
		name    string
		replies map[string]reply
		instr   map[string]any
		wantMsg string
	}{
		{
			name:    "no launch templates",
			replies: happy(),
			instr:   map[string]any{},
			wantMsg: "at least one launch template must be specified",
		},
		{
			name:    "bad skip_matching",
			replies: happy(),
			instr:   map[string]any{"launch_templates": []string{"lt-web"}, "skip_matching": "maybe"},
			wantMsg: "failed to parse skip_matching: ",
		},
		{
			name:    "missing env file",
			replies: happy(),
			instr:   map[string]any{"launch_templates": []string{"lt-web"}, "env": "/nonexistent/opsblade.env"},
			wantMsg: "failed to create AWS client",
		},
		{
			name:    "describe fails",
			replies: map[string]reply{"DescribeAutoScalingGroups": {http.StatusBadRequest, awstest.ASGError("ValidationError", "bad filter")}},
			instr:   map[string]any{"launch_templates": []string{"lt-web"}},
			wantMsg: "error describing autoscaling groups: ",
		},
		{
			name:    "invalid select operator",
			replies: happy(),
			instr:   map[string]any{"launch_templates": []string{"lt-web"}, "select": []map[string]any{{"field": "MinSize", "value": 0, "compare": "matches"}}},
			wantMsg: "failed applying selection criteria: invalid comparison operator",
		},
		{
			name:    "nothing matched",
			replies: happy(),
			instr:   map[string]any{"launch_templates": []string{"lt-unknown"}},
			wantMsg: "no autoscaling groups matched the specified criteria",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, tc.replies)
			r := awstest.Run(t, taskID, shared.TaskContext{}, tc.instr)
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Contains(t, r.Msg, tc.wantMsg)
			assert.Empty(t, r.Data)
			assert.Equal(t, 0, rec.Count("StartInstanceRefresh"))
		})
	}
}

func TestExecute_BadInstructions(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"launch_templates": "lt-web"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}

func TestFoundInList(t *testing.T) {
	cases := []struct {
		name string
		list []string
		item string
		want bool
	}{
		{"exact", []string{"lt-a", "lt-b"}, "lt-b", true},
		{"case insensitive", []string{"LT-A"}, "lt-a", true},
		{"absent", []string{"lt-a"}, "lt-c", false},
		{"empty list", nil, "lt-a", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, foundInList(tc.list, tc.item))
		})
	}
}
