// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package list

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_asg_list"

const describeResponse = `<DescribeAutoScalingGroupsResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeAutoScalingGroupsResult>
    <AutoScalingGroups>
      <member>
        <AutoScalingGroupName>web-asg</AutoScalingGroupName>
        <MinSize>1</MinSize><MaxSize>3</MaxSize><DesiredCapacity>2</DesiredCapacity>
        <LaunchTemplate><LaunchTemplateId>lt-web</LaunchTemplateId><Version>$Default</Version></LaunchTemplate>
      </member>
      <member>
        <AutoScalingGroupName>db-asg</AutoScalingGroupName>
        <MinSize>0</MinSize><MaxSize>1</MaxSize><DesiredCapacity>0</DesiredCapacity>
      </member>
    </AutoScalingGroups>
  </DescribeAutoScalingGroupsResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</DescribeAutoScalingGroupsResponse>`

func serve(t *testing.T, status int, body string) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		if action != "DescribeAutoScalingGroups" {
			return awstest.Unknown(action)
		}
		return status, body
	})
}

func TestExecute_List(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{
		"filters": []map[string]any{{"name": "tag:env", "values": []string{"prod"}}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS Autoscaling Group list", r.Msg)
	assert.Equal(t, 2, r.Data["asg_count"])
	items, isList := r.Data["asg_data"].([]any)
	require.True(t, isList)
	require.Len(t, items, 2)
	assert.Equal(t, "web-asg", items[0].(map[string]any)["AutoScalingGroupName"])
	assert.Equal(t, float64(0), items[1].(map[string]any)["DesiredCapacity"])

	forms := rec.Forms("DescribeAutoScalingGroups")
	require.Len(t, forms, 1)
	assert.Equal(t, "tag:env", forms[0].Get("Filters.member.1.Name"))
	assert.Equal(t, "prod", forms[0].Get("Filters.member.1.Values.member.1"))
	assert.Equal(t, "25", forms[0].Get("MaxRecords"))
}

func TestExecute_SelectAndFields(t *testing.T) {
	serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"select": []map[string]any{{"field": "DesiredCapacity", "value": 1, "compare": "greater"}},
		"fields": []string{"AutoScalingGroupName", "LaunchTemplate.LaunchTemplateId"},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 1, r.Data["asg_count"])
	items := r.Data["asg_data"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]any{
		"AutoScalingGroupName":            "web-asg",
		"LaunchTemplate.LaunchTemplateId": "lt-web",
	}, items[0])
}

func TestExecute_Errors(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		instr   map[string]any
		wantMsg string
	}{
		{
			name: "api error", status: http.StatusBadRequest,
			body:    awstest.ASGError("ValidationError", "1 validation error detected"),
			instr:   map[string]any{},
			wantMsg: "error describing autoscaling groups: ",
		},
		{
			name: "invalid select operator", status: http.StatusOK, body: describeResponse,
			instr:   map[string]any{"select": []map[string]any{{"field": "AutoScalingGroupName", "value": "x", "compare": "matches"}}},
			wantMsg: "failed applying selection criteria: invalid comparison operator",
		},
		{
			name: "missing env file", status: http.StatusOK, body: describeResponse,
			instr:   map[string]any{"env": "/nonexistent/opsblade.env"},
			wantMsg: "failed to create AWS client",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serve(t, tc.status, tc.body)
			r := awstest.Run(t, taskID, shared.TaskContext{}, tc.instr)
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Contains(t, r.Msg, tc.wantMsg)
			assert.Empty(t, r.Data)
		})
	}
}

func TestExecute_BadInstructions(t *testing.T) {
	serve(t, http.StatusOK, describeResponse)
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"fields": "AutoScalingGroupName"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
