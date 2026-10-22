// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

func resetVars(t *testing.T) {
	t.Helper()
	shared.Variables = make(map[string]any)
	t.Cleanup(func() { shared.Variables = make(map[string]any) })
}

func execute(t *testing.T, ctx shared.TaskContext, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx.Task = "cmd_exec"
	ctx.Instructions = raw
	return shared.TaskRegistry["cmd_exec"](ctx).Execute()
}

func TestExecute_Success(t *testing.T) {
	resetVars(t)
	shared.SetVar("greeting", "hello")
	r := execute(t, shared.TaskContext{Debug: true}, map[string]any{"cmd": "echo", "args": []string{"{{greeting}}", "world"}})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "Command executed successfully", r.Msg)
	assert.Equal(t, "echo", r.Data["cmd"])
	assert.Equal(t, []string{"hello", "world"}, r.Data["cmd_args"], "arguments are variable-resolved")
	assert.Equal(t, "hello world\n", r.Data["cmd_output"])
}

func TestExecute_NonZeroExit(t *testing.T) {
	cases := []struct {
		name    string
		noFail  bool
		wantOK  bool
		wantMsg string
	}{
		{"fails by default", false, false, "command execution failed"},
		{"ignored with no_fail", true, true, "ignored because no_fail is set"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := execute(t, shared.TaskContext{}, map[string]any{"cmd": "false", "no_fail": tc.noFail})
			assert.Equal(t, tc.wantOK, r.Success)
			assert.Contains(t, r.Msg, tc.wantMsg)
			if tc.wantOK {
				assert.Equal(t, "false", r.Data["cmd"])
			}
		})
	}
}

func TestExecute_MissingCommand(t *testing.T) {
	r := execute(t, shared.TaskContext{}, map[string]any{"cmd": "/nonexistent/opsblade-test-cmd"})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "command execution failed")
}

func TestExecute_DryRun(t *testing.T) {
	r := execute(t, shared.TaskContext{DryRun: true}, map[string]any{"cmd": "false"})
	require.True(t, r.Success)
	assert.Equal(t, "DryRun, command not executed", r.Msg)
	assert.Empty(t, r.Data)
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "cmd_exec", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["cmd_exec"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
