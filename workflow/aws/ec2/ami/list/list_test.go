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

const taskID = "aws_ec2_ami_list"

const describeResponse = `<DescribeImagesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <imagesSet>
    <item>
      <imageId>ami-old</imageId><imageState>available</imageState><name>web-2025</name>
      <imageOwnerId>111111111111</imageOwnerId><creationDate>2025-01-01T00:00:00.000Z</creationDate>
    </item>
    <item>
      <imageId>ami-new</imageId><imageState>pending</imageState><name>web-2026</name>
      <imageOwnerId>111111111111</imageOwnerId><creationDate>2026-01-01T00:00:00.000Z</creationDate>
    </item>
  </imagesSet>
</DescribeImagesResponse>`

func serve(t *testing.T, status int, body string) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		if action != "DescribeImages" {
			return awstest.Unknown(action)
		}
		return status, body
	})
}

func TestExecute_List(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{
		"owner":   "self",
		"filters": []map[string]any{{"name": "name", "values": []string{"web-*"}}},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS EC2 AMI list", r.Msg)
	assert.Equal(t, 2, r.Data["ami_count"])
	items, ok := r.Data["ami_data"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2)
	assert.Equal(t, "ami-old", items[0].(map[string]any)["ImageId"])
	assert.Equal(t, "pending", items[1].(map[string]any)["State"])

	forms := rec.Forms("DescribeImages")
	require.Len(t, forms, 1)
	assert.Equal(t, "self", forms[0].Get("Owner.1"))
	assert.Equal(t, "name", forms[0].Get("Filter.1.Name"))
	assert.Equal(t, "web-*", forms[0].Get("Filter.1.Value.1"))
}

func TestExecute_NoOwner(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{})
	require.True(t, r.Success, r.Msg)
	forms := rec.Forms("DescribeImages")
	require.Len(t, forms, 1)
	assert.Empty(t, forms[0].Get("Owner.1"))
}

func TestExecute_SelectAndFields(t *testing.T) {
	serve(t, http.StatusOK, describeResponse)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{
		"select": []map[string]any{{"field": "State", "value": "available", "compare": "equal"}},
		"fields": []string{"ImageId", "Name"},
	})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, 1, r.Data["ami_count"])
	items := r.Data["ami_data"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, map[string]any{"ImageId": "ami-old", "Name": "web-2025"}, items[0])
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
			wantMsg: "error describing images: ",
		},
		{
			name: "invalid select operator", status: http.StatusOK, body: describeResponse,
			instr:   map[string]any{"select": []map[string]any{{"field": "ImageId", "value": "x", "compare": "matches"}}},
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
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"fields": "ImageId"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
