// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package describe

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_asg_describe_refreshes"

// describeResponse returns two refreshes for the named group: the newest
// (InProgress) first, then an older Successful one, as the API orders them.
func describeResponse(asg string) string {
	return fmt.Sprintf(`<DescribeInstanceRefreshesResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/">
  <DescribeInstanceRefreshesResult>
    <InstanceRefreshes>
      <member><InstanceRefreshId>%[1]s-new</InstanceRefreshId><AutoScalingGroupName>%[1]s</AutoScalingGroupName>
        <Status>InProgress</Status><PercentageComplete>40</PercentageComplete></member>
      <member><InstanceRefreshId>%[1]s-old</InstanceRefreshId><AutoScalingGroupName>%[1]s</AutoScalingGroupName>
        <Status>Successful</Status><PercentageComplete>100</PercentageComplete></member>
    </InstanceRefreshes>
  </DescribeInstanceRefreshesResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</DescribeInstanceRefreshesResponse>`, asg)
}

func serve(t *testing.T, status int, body string) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, form url.Values) (int, string) {
		if action != "DescribeInstanceRefreshes" {
			return awstest.Unknown(action)
		}
		if body == "" {
			return status, describeResponse(form.Get("AutoScalingGroupName"))
		}
		return status, body
	})
}

func ids(t *testing.T, r shared.TaskResult) []string {
	t.Helper()
	items, isList := r.Data["describe_refreshes"].([]any)
	require.True(t, isList, "describe_refreshes must be a list")
	var out []string
	for _, item := range items {
		out = append(out, item.(map[string]any)["InstanceRefreshId"].(string))
	}
	return out
}

func TestExecute_SingleName(t *testing.T) {
	rec := serve(t, http.StatusOK, "")
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{"asg_name": "web-asg"})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS ASG Describe Refreshes", r.Msg)
	assert.Equal(t, 2, r.Data["describe_refreshes_count"])
	assert.Equal(t, []string{"web-asg-new", "web-asg-old"}, ids(t, r))

	forms := rec.Forms("DescribeInstanceRefreshes")
	require.Len(t, forms, 1)
	assert.Equal(t, "web-asg", forms[0].Get("AutoScalingGroupName"))
	assert.Equal(t, "25", forms[0].Get("MaxRecords"))
}

func TestExecute_ListPlusName(t *testing.T) {
	rec := serve(t, http.StatusOK, "")
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{"asgs": []string{"a-asg", "b-asg"}, "asg_name": "c-asg"})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 6, r.Data["describe_refreshes_count"])
	assert.Equal(t, 3, rec.Count("DescribeInstanceRefreshes"))
	assert.Equal(t, "c-asg", rec.Forms("DescribeInstanceRefreshes")[2].Get("AutoScalingGroupName"))
}

func TestExecute_MostRecent(t *testing.T) {
	serve(t, http.StatusOK, "")
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"asgs":        []string{"a-asg", "b-asg"},
		"most_recent": true,
		"fields":      []string{"InstanceRefreshId", "Status"},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 2, r.Data["describe_refreshes_count"])
	assert.Equal(t, []string{"a-asg-new", "b-asg-new"}, ids(t, r))
	assert.Equal(t, map[string]any{"InstanceRefreshId": "a-asg-new", "Status": "InProgress"},
		r.Data["describe_refreshes"].([]any)[0])
}

func TestExecute_Select(t *testing.T) {
	serve(t, http.StatusOK, "")
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"asg_name": "web-asg",
		"select":   []map[string]any{{"field": "Status", "value": "Successful", "compare": "equal"}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, []string{"web-asg-old"}, ids(t, r))
}

// most_recent looks only at the newest refresh, so a selection that does not
// match it yields nothing rather than falling through to an older refresh.
func TestExecute_MostRecentWithSelect(t *testing.T) {
	serve(t, http.StatusOK, "")
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"asg_name":    "web-asg",
		"most_recent": true,
		"select":      []map[string]any{{"field": "Status", "value": "Successful", "compare": "equal"}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 0, r.Data["describe_refreshes_count"])
	assert.Nil(t, r.Data["describe_refreshes"])
}

func TestExecute_NoGroups(t *testing.T) {
	rec := serve(t, http.StatusOK, "")
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 0, r.Data["describe_refreshes_count"])
	assert.Equal(t, 0, rec.Count("DescribeInstanceRefreshes"))
}

func TestExecute_Errors(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		instr      map[string]any
		wantMsg    string
		wantPrefix string
	}{
		{
			name: "api error", status: http.StatusBadRequest,
			body:    awstest.ASGError("ValidationError", "AutoScalingGroup name not found"),
			instr:   map[string]any{"asg_name": "web-asg"},
			wantMsg: "api error ValidationError", wantPrefix: "error describing instance refreshes: ",
		},
		{
			name: "invalid select operator", status: http.StatusOK, body: "",
			instr:   map[string]any{"asg_name": "web-asg", "select": []map[string]any{{"field": "Status", "value": "x", "compare": "matches"}}},
			wantMsg: "failed applying selection criteria: invalid comparison operator",
		},
		{
			name: "missing env file", status: http.StatusOK, body: "",
			instr:   map[string]any{"asg_name": "web-asg", "env": "/nonexistent/opsblade.env"},
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
			if tc.wantPrefix != "" {
				assert.True(t, strings.HasPrefix(r.Msg, tc.wantPrefix), r.Msg)
			}
			assert.Empty(t, r.Data)
		})
	}
}

func TestExecute_BadInstructions(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"asgs": "web-asg"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
