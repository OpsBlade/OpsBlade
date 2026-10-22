// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package wait

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_ec2_instance_wait"

func describeInstances(state string) string {
	return fmt.Sprintf(`<DescribeInstancesResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <reservationSet><item><reservationId>r-1</reservationId><ownerId>111111111111</ownerId>
    <instancesSet><item><instanceId>i-0123</instanceId>
      <instanceState><code>16</code><name>%s</name></instanceState>
    </item></instancesSet>
  </item></reservationSet>
</DescribeInstancesResponse>`, state)
}

func describeStatus(system, instance string) string {
	return fmt.Sprintf(`<DescribeInstanceStatusResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <instanceStatusSet><item><instanceId>i-0123</instanceId><availabilityZone>us-east-1a</availabilityZone>
    <instanceState><code>16</code><name>running</name></instanceState>
    <systemStatus><status>%s</status></systemStatus>
    <instanceStatus><status>%s</status></instanceStatus>
  </item></instanceStatusSet>
</DescribeInstanceStatusResponse>`, system, instance)
}

// serve answers DescribeInstances with the given state and DescribeInstanceStatus
// with the given check results, delaying the DescribeInstances reply by delay.
func serve(t *testing.T, state, system, instance string, delay time.Duration) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		switch action {
		case "DescribeInstances":
			time.Sleep(delay)
			return http.StatusOK, describeInstances(state)
		case "DescribeInstanceStatus":
			return http.StatusOK, describeStatus(system, instance)
		}
		return awstest.Unknown(action)
	})
}

func TestExecute_Stopped(t *testing.T) {
	rec := serve(t, "stopped", "ok", "ok", 0)
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{"instance_id": "i-0123", "state": "Stopped", "limit": 30})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "instance i-0123 is in state stopped", r.Msg)
	assert.Empty(t, r.Data)
	forms := rec.Forms("DescribeInstances")
	require.Len(t, forms, 1)
	assert.Equal(t, "i-0123", forms[0].Get("InstanceId.1"))
	assert.Equal(t, 0, rec.Count("DescribeInstanceStatus"))
}

func TestExecute_RunningWithChecksPassed(t *testing.T) {
	rec := serve(t, "running", "ok", "ok", 0)
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{"instance_id": "i-0123", "state": "running", "limit": 30})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "instance i-0123 is running and passed all status checks", r.Msg)
	assert.Equal(t, map[string]any{"instance_id": "i-0123"}, r.Data)
	assert.Equal(t, 1, rec.Count("DescribeInstances"))
	forms := rec.Forms("DescribeInstanceStatus")
	require.Len(t, forms, 1)
	assert.Equal(t, "i-0123", forms[0].Get("InstanceId.1"))
	assert.Equal(t, "true", forms[0].Get("IncludeAllInstances"))
}

// A zero limit is exceeded by the time of the first check, so the task must
// report a timeout without calling the API and without sleeping.
func TestExecute_StateTimeout(t *testing.T) {
	rec := serve(t, "pending", "ok", "ok", 0)
	r := awstest.Run(t, taskID, shared.TaskContext{}, map[string]any{"instance_id": "i-0123", "state": "running", "limit": 0})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, "timeout, instance i-0123 not in state running after 0 seconds", r.Msg)
	assert.Equal(t, 0, rec.Count("DescribeInstances"))
}

// The instance reaches running just as the limit expires, so the status-check
// phase must time out before its first API call.
func TestExecute_StatusCheckTimeout(t *testing.T) {
	rec := serve(t, "running", "initializing", "initializing", 1100*time.Millisecond)
	r := awstest.Run(t, taskID, shared.TaskContext{Debug: true}, map[string]any{"instance_id": "i-0123", "state": "running", "limit": 1})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, "timeout, instance i-0123 status checks did not pass after 1 seconds", r.Msg)
	assert.Equal(t, 1, rec.Count("DescribeInstances"))
	assert.Equal(t, 0, rec.Count("DescribeInstanceStatus"))
}

func TestExecute_DryRun(t *testing.T) {
	rec := serve(t, "pending", "ok", "ok", 0)
	r := awstest.Run(t, taskID, shared.TaskContext{DryRun: true, Debug: true}, map[string]any{"instance_id": "i-0123", "state": "running", "limit": 30})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "DryRun, no instance state change", r.Msg)
	assert.Equal(t, 0, rec.Count("DescribeInstances"))
}

func TestExecute_Errors(t *testing.T) {
	cases := []struct {
		name    string
		instr   map[string]any
		wantMsg string
	}{
		{"invalid state", map[string]any{"instance_id": "i-0123", "state": "rebooting"}, "invalid state rebooting"},
		{"empty state", map[string]any{"instance_id": "i-0123"}, "invalid state "},
		{"missing env file", map[string]any{"instance_id": "i-0123", "state": "stopped", "env": "/nonexistent/opsblade.env"}, "failed to create AWS client"},
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
