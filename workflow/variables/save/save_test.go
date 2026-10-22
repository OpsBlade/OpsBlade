// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package save

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	ctx.Task = "variables_save"
	ctx.Instructions = raw
	return shared.TaskRegistry["variables_save"](ctx).Execute()
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var v map[string]any
	require.NoError(t, json.Unmarshal(raw, &v))
	return v
}

func TestExecute_Save(t *testing.T) {
	cases := []struct {
		name   string
		fields []string
		want   map[string]any
	}{
		{"all variables", nil, map[string]any{"a": float64(1), "b": "x"}},
		{"selected fields", []string{"b"}, map[string]any{"b": "x"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetVars(t)
			shared.SetVar("a", 1)
			shared.SetVar("b", "x")
			path := filepath.Join(t.TempDir(), "vars.json")

			instr := map[string]any{"filename": path}
			if tc.fields != nil {
				instr["fields"] = tc.fields
			}
			r := execute(t, shared.TaskContext{Debug: true}, instr)
			require.True(t, r.Success, r.Msg)
			assert.True(t, r.NoVars, "saved data must not be re-imported as variables")
			assert.Contains(t, r.Msg, "Saved variables to "+path)
			assert.Equal(t, tc.want, readJSON(t, path))
			assert.Equal(t, tc.want, r.Data)
		})
	}
}

func TestExecute_FilenameFromVariable(t *testing.T) {
	resetVars(t)
	path := filepath.Join(t.TempDir(), "vars.json")
	shared.SetVar("out", path)
	r := execute(t, shared.TaskContext{}, map[string]any{"filename": "{{out}}"})
	require.True(t, r.Success, r.Msg)
	assert.FileExists(t, path)
}

func TestExecute_DryRun(t *testing.T) {
	resetVars(t)
	path := filepath.Join(t.TempDir(), "vars.json")
	r := execute(t, shared.TaskContext{DryRun: true}, map[string]any{"filename": path})
	require.True(t, r.Success)
	assert.Contains(t, r.Msg, "Dry run: would save variables to "+path)
	assert.NoFileExists(t, path)
}

func TestExecute_CreateError(t *testing.T) {
	resetVars(t)
	path := filepath.Join(t.TempDir(), "missing", "vars.json")
	r := execute(t, shared.TaskContext{}, map[string]any{"filename": path})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to create file")
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "variables_save", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["variables_save"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
