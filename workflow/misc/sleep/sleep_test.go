// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package sleep

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

func execute(t *testing.T, ctx shared.TaskContext, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx.Task = "sleep"
	ctx.Instructions = raw
	return shared.TaskRegistry["sleep"](ctx).Execute()
}

func TestExecute_Sleeps(t *testing.T) {
	start := time.Now()
	r := execute(t, shared.TaskContext{Debug: true}, map[string]any{"sleep": 1})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "Slept for 1 seconds", r.Msg)
	assert.GreaterOrEqual(t, time.Since(start), time.Second)
}

func TestExecute_DryRunDoesNotSleep(t *testing.T) {
	start := time.Now()
	r := execute(t, shared.TaskContext{DryRun: true}, map[string]any{"sleep": 30})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "Dry run, would sleep for 30 seconds", r.Msg)
	assert.Less(t, time.Since(start), time.Second)
}

func TestExecute_RejectsShortSleep(t *testing.T) {
	for _, n := range []int{0, -1} {
		r := execute(t, shared.TaskContext{}, map[string]any{"sleep": n})
		assert.False(t, r.Success)
		assert.Contains(t, r.Msg, "Sleep time must be one second or greater")
	}
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "sleep", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["sleep"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
