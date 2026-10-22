// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package dump

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
	ctx := shared.TaskContext{Task: "variables_dump", Instructions: raw}
	return shared.TaskRegistry["variables_dump"](ctx).Execute()
}

func TestExecute_Dump(t *testing.T) {
	cases := []struct {
		name   string
		fields []string
		want   map[string]any
	}{
		{"all variables", nil, map[string]any{"a": float64(1), "b": "x"}},
		{"selected fields", []string{"a"}, map[string]any{"a": float64(1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetVars(t)
			shared.SetVar("a", 1)
			shared.SetVar("b", "x")
			instr := map[string]any{}
			if tc.fields != nil {
				instr["fields"] = tc.fields
			}
			r := execute(t, instr)
			require.True(t, r.Success, r.Msg)
			assert.Equal(t, "variables attached", r.Msg)
			assert.Equal(t, tc.want, r.Data)
		})
	}
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "variables_dump", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["variables_dump"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
