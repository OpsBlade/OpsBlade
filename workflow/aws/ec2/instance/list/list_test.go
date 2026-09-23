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

const taskID = "aws_ec2_instance_list"

const describeResponse = `<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <reservationSet>
    <item>
      <reservationId>r-1</reservationId>
      <ownerId>111111111111</ownerId>
      <instancesSet>
        <item>
          <instanceId>i-web</instanceId>
          <instanceType>t3.micro</instanceType>
          <instanceState><code>16</code><name>running</name></instanceState>
          <tagSet><item><key>Name</key><value>web</value></item></tagSet>
        </item>
        <item>
          <instanceId>i-db</instanceId>
          <instanceType>m5.large</instanceType>
          <instanceState><code>80</code><name>stopped</name></instanceState>
          <tagSet><item><key>Name</key><value>db</value></item></tagSet>
        </item>
      </instancesSet>
    </item>
  </reservationSet>
</DescribeInstancesResponse>`

func serve(t *testing.T, status int, body string) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		if action != "DescribeInstances" {
			return awstest.Unknown(action)
		}
		return status, body
	})
}

func TestExecute_List(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{
		"owner":   "self",
		"filters": []map[string]any{{"name": "instance-state-name", "values": []string{"running", "stopped"}}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS EC2 Instance list", r.Msg)
	assert.Equal(t, 2, r.Data["instance_count"])
	items, ok := r.Data["instance_data"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "i-web", first["InstanceId"])
	assert.Equal(t, "running", first["State"].(map[string]any)["Name"])

	forms := rec.Forms("DescribeInstances")
	require.Len(t, forms, 1)
	assert.Equal(t, "instance-state-name", forms[0].Get("Filter.1.Name"))
	assert.Equal(t, "running", forms[0].Get("Filter.1.Value.1"))
	assert.Equal(t, "stopped", forms[0].Get("Filter.1.Value.2"))
	assert.Equal(t, "owner-id", forms[0].Get("Filter.2.Name"))
	assert.Equal(t, "self", forms[0].Get("Filter.2.Value.1"))
}

func TestExecute_SelectAndFields(t *testing.T) {
	serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"select": []map[string]any{{"field": "State.Name", "value": "stopped", "compare": "equal"}},
		"fields": []string{"InstanceId", "InstanceType"},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 1, r.Data["instance_count"])
	items := r.Data["instance_data"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]any{"InstanceId": "i-db", "InstanceType": "m5.large"}, items[0])
}

func TestExecute_SelectNothing(t *testing.T) {
	serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"select": []map[string]any{{"field": "InstanceId", "value": "i-none", "compare": "equal"}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 0, r.Data["instance_count"])
	assert.Nil(t, r.Data["instance_data"])
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
			body:    awstest.EC2Error("InvalidParameterValue", "bad filter"),
			instr:   map[string]any{},
			wantMsg: "error describing instances: ",
		},
		{
			name: "invalid select operator", status: http.StatusOK, body: describeResponse,
			instr:   map[string]any{"select": []map[string]any{{"field": "InstanceId", "value": "x", "compare": "matches"}}},
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
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"fields": "InstanceId"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
