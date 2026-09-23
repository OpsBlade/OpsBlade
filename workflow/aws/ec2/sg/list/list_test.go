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

const taskID = "aws_ec2_sg_list"

const describeResponse = `<DescribeSecurityGroupsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <securityGroupInfo>
    <item>
      <ownerId>111111111111</ownerId><groupId>sg-web</groupId><groupName>web</groupName>
      <groupDescription>web servers</groupDescription><vpcId>vpc-1</vpcId>
    </item>
    <item>
      <ownerId>111111111111</ownerId><groupId>sg-db</groupId><groupName>db</groupName>
      <groupDescription>databases</groupDescription><vpcId>vpc-1</vpcId>
    </item>
  </securityGroupInfo>
</DescribeSecurityGroupsResponse>`

func serve(t *testing.T, status int, body string) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		if action != "DescribeSecurityGroups" {
			return awstest.Unknown(action)
		}
		return status, body
	})
}

func TestExecute_List(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{
		"filters": []map[string]any{{"name": "vpc-id", "values": []string{"vpc-1"}}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS EC2 Security Group list", r.Msg)
	assert.Equal(t, 2, r.Data["security_group_count"])
	items, ok := r.Data["security_group_data"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	assert.Equal(t, "sg-web", items[0].(map[string]any)["GroupId"])
	assert.Equal(t, "db", items[1].(map[string]any)["GroupName"])

	forms := rec.Forms("DescribeSecurityGroups")
	require.Len(t, forms, 1)
	assert.Equal(t, "vpc-id", forms[0].Get("Filter.1.Name"))
	assert.Equal(t, "vpc-1", forms[0].Get("Filter.1.Value.1"))
}

func TestExecute_SelectAndFields(t *testing.T) {
	serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"select": []map[string]any{{"field": "GroupName", "value": "d", "compare": "begins"}},
		"fields": []string{"GroupId"},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 1, r.Data["security_group_count"])
	items := r.Data["security_group_data"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]any{"GroupId": "sg-db"}, items[0])
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
			body:    awstest.EC2Error("InvalidGroup.NotFound", "The security group does not exist"),
			instr:   map[string]any{},
			wantMsg: "api error InvalidGroup.NotFound",
		},
		{
			name: "invalid select operator", status: http.StatusOK, body: describeResponse,
			instr:   map[string]any{"select": []map[string]any{{"field": "GroupId", "value": "x", "compare": "matches"}}},
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
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"fields": "GroupId"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
