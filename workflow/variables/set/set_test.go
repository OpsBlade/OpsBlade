// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package load

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

func execute(t *testing.T, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx := shared.TaskContext{Task: "variables_set", Instructions: raw}
	return shared.TaskRegistry["variables_set"](ctx).Execute()
}

func TestExecute_SetsVariables(t *testing.T) {
	resetVars(t)
	shared.SetVar("base", "v")
	r := execute(t, map[string]any{"set": []map[string]any{
		{"name": "a", "value": 1},
		{"name": "b", "value": "{{base}}-1"},
	}})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "variables set", r.Msg)
	assert.Equal(t, map[string]any{"a": float64(1), "b": "v-1"}, r.Data)
	assert.Equal(t, float64(1), shared.GetVar("a"))
	assert.Equal(t, "v-1", shared.GetVar("b"), "values are resolved before being stored")
}

func TestExecute_NoVariables(t *testing.T) {
	resetVars(t)
	r := execute(t, map[string]any{})
	require.True(t, r.Success)
	assert.Empty(t, r.Data)
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "variables_set", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["variables_set"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
