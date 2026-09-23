// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func fieldsTestDoc() map[string]any {
	return map[string]any{
		"Name": "n",
		"Tags": []any{
			map[string]any{"Key": "k1", "Value": "v1"},
			map[string]any{"Key": "k2", "Value": "v2"},
		},
		"Nested": map[string]any{"A": "a", "B": map[string]any{"C": "c"}},
		"Nums":   []any{1.0, 2.0},
	}
}

func TestSelectFields(t *testing.T) {
	tests := []struct {
		name   string
		fields []string
		want   map[string]any
	}{
		{
			name:   "top level case insensitive",
			fields: []string{"name"},
			want:   map[string]any{"Name": "n"},
		},
		{
			name:   "wildcard over list",
			fields: []string{"Tags.*.Key"},
			want: map[string]any{"Tags": []map[string]any{
				{"Key": "k1"},
				{"Key": "k2"},
			}},
		},
		{
			name:   "two wildcard fields on same list",
			fields: []string{"tags.*.key", "tags.*.value"},
			want: map[string]any{"Tags": []map[string]any{
				{"Key": "k1", "Value": "v1"},
				{"Key": "k2", "Value": "v2"},
			}},
		},
		{
			name:   "dotted path into map",
			fields: []string{"Nested.A"},
			want:   map[string]any{"Nested.A": "a"},
		},
		{
			name:   "deep dotted path",
			fields: []string{"nested.b.c"},
			want:   map[string]any{"Nested.B.C": "c"},
		},
		{
			name:   "indexed list element",
			fields: []string{"Nums.0"},
			want:   map[string]any{"Nums.0": 1.0},
		},
		{
			name:   "indexed list element field",
			fields: []string{"Tags.0.Key"},
			want:   map[string]any{"Tags.0.Key": "k1"},
		},
		{
			name:   "whole list",
			fields: []string{"Nums"},
			want:   map[string]any{"Nums": []any{1.0, 2.0}},
		},
		{
			name:   "parent wins over child",
			fields: []string{"Nested", "Nested.A"},
			want:   map[string]any{"Nested": map[string]any{"A": "a", "B": map[string]any{"C": "c"}}},
		},
		{
			name:   "missing field",
			fields: []string{"missing"},
			want:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SelectFields(fieldsTestDoc(), tt.fields))
		})
	}
}

func TestSelectFieldsNoFieldsReturnsWholeDocument(t *testing.T) {
	got := SelectFields(fieldsTestDoc(), nil)
	assert.Equal(t, "n", got["Name"])
	assert.Len(t, got, 4)
}

func TestSelectFieldsStructInput(t *testing.T) {
	type tag struct {
		Key   string
		Value string
	}
	type doc struct {
		Name string
		Tags []tag
	}
	input := doc{Name: "x", Tags: []tag{{Key: "a", Value: "b"}}}

	got := SelectFields(input, []string{"name", "tags.*.value"})
	assert.Equal(t, map[string]any{
		"Name": "x",
		"Tags": []map[string]any{{"Value": "b"}},
	}, got)
}

func TestSelectFieldsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{name: "unserializable", input: make(chan int)},
		{name: "not an object", input: "str"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, map[string]any{}, SelectFields(tt.input, []string{"a"}))
		})
	}
}

func TestCollectFieldsDirectStructAndSlice(t *testing.T) {
	type inner struct {
		Name   string
		hidden string //nolint:unused // verifies unexported fields are skipped
	}
	type outer struct {
		Items []inner
		Pairs map[string]inner
		Flag  bool
		Skip  []string
	}
	input := outer{
		Items: []inner{{Name: "a"}, {Name: "b"}},
		Pairs: map[string]inner{"x": {Name: "px"}},
		Flag:  true,
		Skip:  []string{"s"},
	}

	output := map[string]any{}
	collectFields("", input, []string{"Items.Name", "Pairs.Name", "flag"}, output)

	assert.Equal(t, true, output["Flag"])
	assert.Equal(t, []any{"a", "b"}, output["Items.Name"])
	assert.Equal(t, map[string]any{"x": "px"}, output["Pairs.Name"])
}

func TestCollectFieldsSliceWithNoFields(t *testing.T) {
	output := map[string]any{}
	collectFields("", []any{"a"}, nil, output)
	assert.Empty(t, output)
}

func TestContainsFieldIgnoreCase(t *testing.T) {
	fields := []string{"Name", "tags.*.Key"}
	assert.True(t, containsFieldIgnoreCase(fields, "name"))
	assert.True(t, containsFieldIgnoreCase(fields, "TAGS.*.key"))
	assert.False(t, containsFieldIgnoreCase(fields, "value"))
	assert.False(t, containsFieldIgnoreCase(nil, "name"))
}

func TestJoinPath(t *testing.T) {
	assert.Equal(t, "a", joinPath("", "a"))
	assert.Equal(t, "a.b", joinPath("a", "b"))
}

func TestRemoveFirstWildcard(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "a.*.b.c", want: "b.c"},
		{in: "a.*.b.*.c", want: "b.*.c"},
		{in: "a.b", want: "a.b"},
		{in: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, removeFirstWildcard(tt.in))
		})
	}
}

func TestRemoveConflictingFields(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "child removed", in: []string{"Ami", "ami.name"}, want: []string{"ami"}},
		{name: "no conflict", in: []string{"a", "b.c"}, want: []string{"a", "b.c"}},
		{name: "duplicates collapse", in: []string{"A", "a"}, want: []string{"a"}},
		{name: "prefix without dot is not a conflict", in: []string{"ami", "amiName"}, want: []string{"ami", "aminame"}},
		{name: "empty", in: nil, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeConflictingFields(tt.in)
			sort.Strings(got)
			assert.Equal(t, tt.want, got)
		})
	}
}
