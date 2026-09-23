// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package stop

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_ec2_instance_stop"

const stopResponse = `<StopInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <instancesSet><item><instanceId>i-0123</instanceId>
    <currentState><code>64</code><name>stopping</name></currentState>
    <previousState><code>16</code><name>running</name></previousState>
  </item></instancesSet>
</StopInstancesResponse>`

func TestExecute(t *testing.T) {
	cases := []struct {
		name     string
		dryRun   bool
		force    bool
		status   int
		body     string
		wantOK   bool
		wantMsg  string
		wantData map[string]any
	}{
		{
			name: "stopping", status: http.StatusOK, body: stopResponse,
			wantOK: true, wantMsg: "AWS EC2 Instance stopping", wantData: map[string]any{"instance_id": "i-0123"},
		},
		{
			name: "forced", force: true, status: http.StatusOK, body: stopResponse,
			wantOK: true, wantMsg: "AWS EC2 Instance stopping", wantData: map[string]any{"instance_id": "i-0123"},
		},
		{
			name: "api error", status: http.StatusBadRequest,
			body:    awstest.EC2Error("IncorrectInstanceState", "The instance is not in a state from which it can be stopped"),
			wantMsg: "api error IncorrectInstanceState",
		},
		{
			name: "dry run accepted", dryRun: true, status: http.StatusPreconditionFailed,
			body:   awstest.EC2Error(awstest.DryRunCode, "Request would have succeeded, but DryRun flag is set."),
			wantOK: true, wantMsg: "Dryrun, AWS API returned: ",
		},
		{
			name: "dry run with real error", dryRun: true, status: http.StatusForbidden,
			body:    awstest.EC2Error("UnauthorizedOperation", "You are not authorized to perform this operation."),
			wantMsg: "api error UnauthorizedOperation",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := awstest.Serve(t, func(action string, _ url.Values) (int, string) {
				if action != "StopInstances" {
					return awstest.Unknown(action)
				}
				return tc.status, tc.body
			})
			r := awstest.Run(t, taskID, shared.TaskContext{DryRun: tc.dryRun, Debug: true},
				map[string]any{"instance_id": "i-0123", "force": tc.force})
			assert.Equal(t, tc.wantOK, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Contains(t, r.Msg, tc.wantMsg)
			if tc.wantData == nil {
				assert.Empty(t, r.Data)
			} else {
				assert.Equal(t, tc.wantData, r.Data)
			}

			forms := rec.Forms("StopInstances")
			require.Len(t, forms, 1)
			assert.Equal(t, "i-0123", forms[0].Get("InstanceId.1"))
			assert.Equal(t, boolString(tc.force), forms[0].Get("Force"))
			assert.Equal(t, boolString(tc.dryRun), forms[0].Get("DryRun"))
		})
	}
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestExecute_BadInstructions(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"force": "yes"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}

func TestExecute_MissingEnvFile(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{"instance_id": "i-0123", "env": t.TempDir() + "/missing.env"})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to create AWS client")
}
