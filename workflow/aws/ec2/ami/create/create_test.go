// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package create

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_ec2_ami_create"

const createResponse = `<CreateImageResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId><imageId>ami-new</imageId>
</CreateImageResponse>`

func TestExecute(t *testing.T) {
	cases := []struct {
		name     string
		dryRun   bool
		status   int
		body     string
		wantOK   bool
		wantMsg  string
		wantData map[string]any
	}{
		{
			name: "created", status: http.StatusOK, body: createResponse,
			wantOK: true, wantMsg: "AWS AMI ami-new created", wantData: map[string]any{"image_id": "ami-new"},
		},
		{
			name: "api error", status: http.StatusBadRequest,
			body:    awstest.EC2Error("InvalidInstanceID.NotFound", "The instance ID 'i-0123' does not exist"),
			wantMsg: "error creating image: ",
		},
		{
			name: "dry run accepted", dryRun: true, status: http.StatusPreconditionFailed,
			body:   awstest.EC2Error(awstest.DryRunCode, "Request would have succeeded, but DryRun flag is set."),
			wantOK: true, wantMsg: "Dryrun, AWS API returned: ", wantData: map[string]any{"image_id": "AMI-none-dry-run"},
		},
		{
			name: "dry run with real error", dryRun: true, status: http.StatusForbidden,
			body:    awstest.EC2Error("UnauthorizedOperation", "You are not authorized to perform this operation."),
			wantMsg: "error creating image: ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := awstest.Serve(t, func(action string, _ url.Values) (int, string) {
				if action != "CreateImage" {
					return awstest.Unknown(action)
				}
				return tc.status, tc.body
			})
			r := awstest.Run(t, taskID, shared.TaskContext{DryRun: tc.dryRun, Debug: true}, map[string]any{
				"instance_id":   "i-0123",
				"instance_name": "web-2026",
				"description":   "nightly",
				"no_reboot":     true,
				"tags":          map[string]string{"env": "prod"},
			})
			assert.Equal(t, tc.wantOK, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Contains(t, r.Msg, tc.wantMsg)
			if tc.wantData == nil {
				assert.Empty(t, r.Data)
			} else {
				assert.Equal(t, tc.wantData, r.Data)
			}

			forms := rec.Forms("CreateImage")
			require.Len(t, forms, 1)
			f := forms[0]
			assert.Equal(t, "i-0123", f.Get("InstanceId"))
			assert.Equal(t, "web-2026", f.Get("Name"))
			assert.Equal(t, "nightly", f.Get("Description"))
			assert.Equal(t, "true", f.Get("NoReboot"))
			assert.Equal(t, "image", f.Get("TagSpecification.1.ResourceType"))
			assert.Equal(t, "env", f.Get("TagSpecification.1.Tag.1.Key"))
			assert.Equal(t, "prod", f.Get("TagSpecification.1.Tag.1.Value"))
			if tc.dryRun {
				assert.Equal(t, "true", f.Get("DryRun"))
			} else {
				assert.Equal(t, "false", f.Get("DryRun"))
			}
		})
	}
}

func TestExecute_BadInstructions(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"tags": "env=prod"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data to Task")
}

func TestExecute_MissingEnvFile(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{"instance_id": "i-0123", "env": t.TempDir() + "/missing.env"})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to create AWS client")
}
