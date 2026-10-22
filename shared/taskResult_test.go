// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskResultSerialize(t *testing.T) {
	tr := TaskResult{MessageType: "task_stop", Success: true, Stop: true, Outcome: "stop", Msg: "m", Sequence: 1, Name: "n", Task: "t", Data: map[string]any{"a": "b"}}

	var got TaskResult
	require.NoError(t, json.Unmarshal([]byte(tr.Serialize()), &got))
	assert.Equal(t, tr, got)

	pretty := tr.SerializePretty()
	assert.Contains(t, pretty, "\n  \"success\": true,\n")
	require.NoError(t, json.Unmarshal([]byte(pretty), &got))
	assert.Equal(t, tr, got)
}

func TestTaskResultSerializeExcludesInternalFields(t *testing.T) {
	tr := TaskResult{OnFail: OnFailWarn, NoVars: true, Sequence: 1}
	s := tr.Serialize()
	assert.NotContains(t, s, "warn")
	assert.NotContains(t, s, "NoVars")
}

func TestTaskResultSerializeError(t *testing.T) {
	tr := TaskResult{Success: true, Sequence: 2, Name: "n", Task: "t", Data: map[string]any{"f": func() {}}}

	var got TaskResult
	require.NoError(t, json.Unmarshal([]byte(tr.Serialize()), &got))
	assert.False(t, got.Success)
	assert.Equal(t, 2, got.Sequence)
	assert.Equal(t, "n", got.Name)
	assert.Equal(t, "t", got.Task)
	assert.Nil(t, got.Data)
	assert.Contains(t, got.Msg, "error serializing task result")
}

func TestTaskResultString(t *testing.T) {
	tests := []struct {
		name   string
		result TaskResult
		want   string
	}{
		{
			name:   "without name",
			result: TaskResult{Sequence: 1, Task: "t", Msg: "m"},
			want:   "* Completed task 1: [t]\nSuccess: false\nMessage: m",
		},
		{
			name:   "with name and outcome",
			result: TaskResult{Sequence: 2, Name: "n", Task: "t", Success: true, Outcome: "success", Msg: "m"},
			want:   "* Completed task 2: \"n\" [t]\nSuccess: true\nOutcome: success\nMessage: m",
		},
		{
			name:   "with data",
			result: TaskResult{Sequence: 3, Task: "t", Msg: "m", Data: map[string]any{"a": "b"}},
			want:   "* Completed task 3: [t]\nSuccess: false\nMessage: m\nData:\n  a: b",
		},
		{
			name:   "with empty data",
			result: TaskResult{Sequence: 4, Task: "t", Data: map[string]any{}},
			want:   "* Completed task 4: [t]\nSuccess: false\nMessage: \nData: none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.result.String())
		})
	}
}
