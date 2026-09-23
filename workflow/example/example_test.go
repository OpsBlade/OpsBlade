// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package example

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

func execute(t *testing.T, ctx shared.TaskContext, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx.Task = "example"
	ctx.Instructions = raw
	return shared.TaskRegistry["example"](ctx).Execute()
}

func TestExecute_SelectAndFields(t *testing.T) {
	cases := []struct {
		name         string
		instructions map[string]any
		wantSelected int
		wantKeys     []string // keys expected on the first selected item; nil means the full item
	}{
		{"no criteria returns everything", map[string]any{}, 5, nil},
		{"select filters items", map[string]any{
			"select": []map[string]any{{"field": "paid", "value": "yes", "compare": "equal"}},
		}, 3, nil},
		{"fields limit output", map[string]any{
			"select": []map[string]any{{"field": "tags.tag1", "value": "tag_value2", "compare": "equal"}},
			"fields": []string{"name"},
		}, 1, []string{"name"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := execute(t, shared.TaskContext{Debug: true}, tc.instructions)
			require.True(t, r.Success, r.Msg)
			assert.Equal(t, "example list", r.Msg)
			assert.Equal(t, 5, r.Data["mock_api_items"])
			assert.Equal(t, tc.wantSelected, r.Data["mock_selected_items"])
			items, ok := r.Data["mock_data"].([]any)
			require.True(t, ok, "mock_data must be a list, got %T", r.Data["mock_data"])
			require.Len(t, items, tc.wantSelected)
			first, ok := items[0].(map[string]any)
			require.True(t, ok)
			if tc.wantKeys != nil {
				keys := make([]string, 0, len(first))
				for k := range first {
					keys = append(keys, k)
				}
				assert.ElementsMatch(t, tc.wantKeys, keys)
			} else {
				assert.Contains(t, first, "tags")
			}
		})
	}
}

func TestExecute_InvalidCriteria(t *testing.T) {
	r := execute(t, shared.TaskContext{}, map[string]any{
		"select": []map[string]any{{"field": "paid", "value": "yes", "compare": "bogus"}},
	})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed applying selection criteria")
	assert.Contains(t, r.Msg, "invalid comparison operator: bogus")
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "example", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["example"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
