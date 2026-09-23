// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskInfoSerialize(t *testing.T) {
	ti := TaskInfo{MessageType: "task_start", Sequence: 1, Name: "n", Task: "t", Instructions: map[string]any{"a": "b"}, Debug: true}

	var got TaskInfo
	require.NoError(t, json.Unmarshal([]byte(ti.Serialize()), &got))
	assert.Equal(t, ti, got)

	pretty := ti.SerializePretty()
	assert.Contains(t, pretty, "\n  \"sequence\": 1,\n")
	require.NoError(t, json.Unmarshal([]byte(pretty), &got))
	assert.Equal(t, ti, got)
}

func TestTaskInfoSerializeError(t *testing.T) {
	ti := TaskInfo{Sequence: 1, Name: "n", Task: "t", Instructions: map[string]any{"f": func() {}}}

	var got TaskInfo
	require.NoError(t, json.Unmarshal([]byte(ti.Serialize()), &got))
	assert.Equal(t, 1, got.Sequence)
	assert.Equal(t, "n", got.Name)
	assert.Equal(t, "t", got.Task)
	assert.Nil(t, got.Instructions)
	assert.Contains(t, got.Msg, "error serializing task info")
}

func TestTaskInfoString(t *testing.T) {
	tests := []struct {
		name string
		info TaskInfo
		want string
	}{
		{
			name: "without name",
			info: TaskInfo{Sequence: 1, Task: "t"},
			want: "* Starting task 1: [t]",
		},
		{
			name: "with name",
			info: TaskInfo{Sequence: 2, Name: "n", Task: "t"},
			want: "* Starting task 2: \"n\" [t]",
		},
		{
			name: "instructions hidden without debug",
			info: TaskInfo{Sequence: 3, Task: "t", Instructions: map[string]any{"a": "b"}},
			want: "* Starting task 3: [t]",
		},
		{
			name: "debug with instructions",
			info: TaskInfo{Sequence: 4, Task: "t", Debug: true, Instructions: map[string]any{"a": "b"}},
			want: "* Starting task 4: [t]\nInstructions:\n  a: b",
		},
		{
			name: "debug with empty instructions",
			info: TaskInfo{Sequence: 5, Task: "t", Debug: true, Instructions: map[string]any{}},
			want: "* Starting task 5: [t]\nInstructions: none",
		},
		{
			name: "debug with nil instructions",
			info: TaskInfo{Sequence: 6, Task: "t", Debug: true},
			want: "* Starting task 6: [t]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.info.String())
		})
	}
}
