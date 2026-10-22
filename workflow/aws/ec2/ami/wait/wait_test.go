// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package wait

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_ec2_ami_wait"

func describeResponse(state string) string {
	return fmt.Sprintf(`<DescribeImagesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <imagesSet><item><imageId>ami-new</imageId><imageState>%s</imageState><name>web</name></item></imagesSet>
</DescribeImagesResponse>`, state)
}

func serve(t *testing.T, status int, body string) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		if action != "DescribeImages" {
			return awstest.Unknown(action)
		}
		return status, body
	})
}

func TestExecute_Available(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse("available"))
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{"image_id": "ami-new", "limit": 30})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS AMI ami-new available", r.Msg)
	assert.Empty(t, r.Data)
	forms := rec.Forms("DescribeImages")
	require.Len(t, forms, 1)
	assert.Equal(t, "ami-new", forms[0].Get("ImageId.1"))
}

// A zero limit is exceeded by the time of the first check, so the task must
// report a timeout without calling the API and without sleeping.
func TestExecute_Timeout(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse("pending"))
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{"image_id": "ami-new", "limit": 0})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, "timeout, image ami-new not available after 0 seconds", r.Msg)
	assert.Equal(t, 0, rec.Count("DescribeImages"))
}

func TestExecute_DryRun(t *testing.T) {
	rec := serve(t, http.StatusOK, describeResponse("pending"))
	r := awstest.Run(t, taskID, shared.TaskContext{DryRun: true}, map[string]any{"image_id": "ami-new", "limit": 30})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "DryRun, no image to wait for", r.Msg)
	assert.Equal(t, 0, rec.Count("DescribeImages"))
}

func TestExecute_Errors(t *testing.T) {
	cases := []struct {
		name    string
		instr   map[string]any
		wantMsg string
	}{
		{"missing image id", map[string]any{"limit": 30}, "image_id is null, nothing to wait for"},
		{"missing env file", map[string]any{"image_id": "ami-new", "env": "/nonexistent/opsblade.env"}, "failed to create AWS client"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			awstest.Serve(t, awstest.Reject)
			r := awstest.Run(t, taskID, shared.TaskContext{}, tc.instr)
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Contains(t, r.Msg, tc.wantMsg)
		})
	}
}

func TestExecute_BadInstructions(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"limit": "30"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data to Task")
}
