// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package exitIf

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
	ctx := shared.TaskContext{Task: "exit_if", Instructions: raw, Debug: true}
	return shared.TaskRegistry["exit_if"](ctx).Execute()
}

func TestExecute(t *testing.T) {
	cases := []struct {
		name     string
		value    any
		wantStop bool
	}{
		{"condition met stops", "prod", true},
		{"condition not met continues", "dev", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetVars(t)
			shared.SetVar("stage", tc.value)
			r := execute(t, map[string]any{
				"select": []map[string]any{{"field": "stage", "value": "prod", "compare": "equal"}},
			})
			require.True(t, r.Success, r.Msg)
			assert.Equal(t, tc.wantStop, r.Stop)
			assert.Equal(t, tc.wantStop, r.Data["exit_if_result"])
		})
	}
}

func TestExecute_InvalidCriteria(t *testing.T) {
	resetVars(t)
	r := execute(t, map[string]any{
		"select": []map[string]any{{"field": "stage", "value": "prod", "compare": "bogus"}},
	})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed applying selection criteria")
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "exit_if", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["exit_if"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
