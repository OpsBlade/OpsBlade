// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package load

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
	ctx.Task = "variables_load"
	ctx.Instructions = raw
	return shared.TaskRegistry["variables_load"](ctx).Execute()
}

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vars.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestExecute_Load(t *testing.T) {
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
			path := writeFile(t, `{"a": 1, "b": "x"}`)
			instr := map[string]any{"filename": path}
			if tc.fields != nil {
				instr["fields"] = tc.fields
			}
			r := execute(t, shared.TaskContext{Debug: true}, instr)
			require.True(t, r.Success, r.Msg)
			assert.Contains(t, r.Msg, "Loaded variables from "+path)
			assert.Equal(t, tc.want, r.Data)
		})
	}
}

func TestExecute_FilenameFromVariable(t *testing.T) {
	resetVars(t)
	path := writeFile(t, `{"a": 1}`)
	shared.SetVar("in", path)
	r := execute(t, shared.TaskContext{}, map[string]any{"filename": "{{in}}"})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, float64(1), r.Data["a"])
}

func TestExecute_Errors(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantMsg string
	}{
		{"missing file", filepath.Join(t.TempDir(), "nope.json"), "failed to open file"},
		{"bad json", writeFile(t, "{not json"), "failed to deserialize variables"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetVars(t)
			r := execute(t, shared.TaskContext{}, map[string]any{"filename": tc.path})
			assert.False(t, r.Success)
			assert.Contains(t, r.Msg, tc.wantMsg)
		})
	}
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "variables_load", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["variables_load"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
