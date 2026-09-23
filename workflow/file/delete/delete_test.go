// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package delete

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
	ctx.Task = "file_delete"
	ctx.Instructions = raw
	return shared.TaskRegistry["file_delete"](ctx).Execute()
}

func tempFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "victim.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o600))
	return path
}

func TestExecute_Deletes(t *testing.T) {
	path := tempFile(t)
	r := execute(t, shared.TaskContext{}, map[string]any{"filename": path})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "Deleted "+path, r.Msg)
	assert.NoFileExists(t, path)
}

func TestExecute_FilenameFromVariable(t *testing.T) {
	resetVars(t)
	path := tempFile(t)
	shared.SetVar("victim", path)
	r := execute(t, shared.TaskContext{}, map[string]any{"filename": "{{victim}}"})
	require.True(t, r.Success, r.Msg)
	assert.NoFileExists(t, path)
}

func TestExecute_DryRun(t *testing.T) {
	path := tempFile(t)
	r := execute(t, shared.TaskContext{DryRun: true}, map[string]any{"filename": path})
	require.True(t, r.Success, r.Msg)
	assert.Contains(t, r.Msg, "Dry run: would delete file: "+path)
	assert.FileExists(t, path)
}

func TestExecute_Errors(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		wantMsg string
	}{
		{"empty filename", "", "unable to delete, filename is empty"},
		{"missing file", filepath.Join(t.TempDir(), "nope.txt"), "failed delete file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := execute(t, shared.TaskContext{}, map[string]any{"filename": tc.path})
			assert.False(t, r.Success)
			assert.Contains(t, r.Msg, tc.wantMsg)
		})
	}
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "file_delete", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["file_delete"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
