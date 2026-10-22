// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnyToString(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "nil", value: nil, want: ""},
		{name: "string", value: "abc", want: "abc"},
		{name: "int", value: 42, want: "42"},
		{name: "int64", value: int64(-7), want: "-7"},
		{name: "float64", value: 1.5, want: "1.5"},
		{name: "float64 whole", value: 3.0, want: "3"},
		{name: "bool", value: true, want: "true"},
		{name: "other type", value: uint8(9), want: "9"},
		{name: "slice", value: []int{1, 2, 3}, want: "1,2,3"},
		{name: "array", value: [2]string{"a", "b"}, want: "a,b"},
		{name: "empty slice", value: []string{}, want: ""},
		{name: "map single entry", value: map[string]int{"a": 1}, want: "a:1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToString(tt.value))
		})
	}
}

func TestAnyToStringMapMultipleEntries(t *testing.T) {
	got := AnyToString(map[string]string{"a": "1", "b": "2"})
	assert.Contains(t, []string{"a:1,b:2", "b:2,a:1"}, got)
}

func TestAnyToInt(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int
	}{
		{name: "int", value: 5, want: 5},
		{name: "int64", value: int64(6), want: 6},
		{name: "float64 truncates", value: 7.9, want: 7},
		{name: "numeric string", value: "8", want: 8},
		{name: "invalid string", value: "eight", want: 0},
		{name: "nil", value: nil, want: 0},
		{name: "bool", value: true, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToInt(tt.value))
		})
	}
}

func TestAnyToInt64(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int64
	}{
		{name: "int", value: 5, want: 5},
		{name: "int64", value: int64(6), want: 6},
		{name: "float64 truncates", value: 7.9, want: 7},
		{name: "numeric string", value: "9000000000", want: 9000000000},
		{name: "invalid string", value: "x", want: 0},
		{name: "nil", value: nil, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToInt64(tt.value))
		})
	}
}

func TestAnyToBool(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{name: "true", value: true, want: true},
		{name: "false", value: false, want: false},
		{name: "string true", value: "true", want: true},
		{name: "string 1", value: "1", want: true},
		{name: "string invalid", value: "maybe", want: false},
		{name: "int nonzero", value: 3, want: true},
		{name: "int zero", value: 0, want: false},
		{name: "float nonzero", value: 0.5, want: true},
		{name: "float zero", value: 0.0, want: false},
		{name: "nil", value: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToBool(tt.value))
		})
	}
}

func TestAnyToMapString(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  map[string]string
	}{
		{name: "nil", value: nil, want: map[string]string{}},
		{name: "slice", value: []int{1, 2}, want: map[string]string{"0": "1", "1": "2"}},
		{name: "array", value: [1]string{"a"}, want: map[string]string{"0": "a"}},
		{name: "map string string", value: map[string]string{"a": "b"}, want: map[string]string{"a": "b"}},
		{name: "map string any", value: map[string]any{"a": 1, "b": "x"}, want: map[string]string{"a": "1", "b": "x"}},
		{name: "map string int", value: map[string]int{"a": 1}, want: map[string]string{"a": "1"}},
		{name: "map string int64", value: map[string]int64{"a": 2}, want: map[string]string{"a": "2"}},
		{name: "map string bool", value: map[string]bool{"a": true}, want: map[string]string{"a": "true"}},
		{name: "string", value: "s", want: map[string]string{"0": "s"}},
		{name: "int", value: 3, want: map[string]string{"0": "3"}},
		{name: "int64", value: int64(4), want: map[string]string{"0": "4"}},
		{name: "float64", value: 1.25, want: map[string]string{"0": "1.25"}},
		{name: "bool", value: false, want: map[string]string{"0": "false"}},
		{name: "unsupported type", value: uint(1), want: map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToMapString(tt.value))
		})
	}
}

func TestAnyToMapAny(t *testing.T) {
	type inner struct {
		A string
		B []int
		c int //nolint:unused // verifies unexported fields are skipped
		M map[string]int
	}

	tests := []struct {
		name  string
		value any
		want  map[string]any
	}{
		{name: "nil", value: nil, want: map[string]any{}},
		{name: "map string any returned as is", value: map[string]any{"k": "v"}, want: map[string]any{"k": "v"}},
		{name: "map string int", value: map[string]int{"x": 1}, want: map[string]any{"x": 1}},
		{name: "map int key", value: map[int]string{1: "a"}, want: map[string]any{"1": "a"}},
		{
			name:  "map of structs",
			value: map[string]inner{"x": {A: "a"}},
			want:  map[string]any{"x": map[string]any{"A": "a", "B": []any{}, "M": map[string]any{}}},
		},
		{name: "map of slices", value: map[string][]string{"x": {"a"}}, want: map[string]any{"x": []any{"a"}}},
		{
			name:  "slice",
			value: []any{1, "a", []int{2}, map[string]any{"k": "v"}, nil},
			want:  map[string]any{"0": 1, "1": "a", "2": []any{2}, "3": map[string]any{"k": "v"}, "4": nil},
		},
		{
			name:  "struct",
			value: inner{A: "a", B: []int{1}, M: map[string]int{"x": 1}},
			want:  map[string]any{"A": "a", "B": []any{1}, "M": map[string]any{"x": 1}},
		},
		{name: "scalar", value: 42, want: map[string]any{"0": 42}},
		{name: "nil map value", value: map[string]int64{"k": 0}, want: map[string]any{"k": int64(0)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToMapAny(tt.value))
		})
	}
}

func TestAnyToList(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{name: "nil", value: nil, want: nil},
		{name: "slice", value: []int{1, 2}, want: []string{"1", "2"}},
		{name: "array", value: [2]bool{true, false}, want: []string{"true", "false"}},
		{name: "map", value: map[string]int{"a": 1}, want: []string{"a:1"}},
		{name: "scalar", value: 3, want: []string{"3"}},
		{name: "string", value: "s", want: []string{"s"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToList(tt.value))
		})
	}
}

func TestAnyToYAML(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "map with list", value: map[string]any{"a": 1, "b": []string{"x"}}, want: "a: 1\nb:\n    - x\n"},
		{name: "nil", value: nil, want: "null\n"},
		{name: "unserializable", value: make(chan int), want: "unable to serialize to JSON: json: unsupported type: chan int"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToYAML(tt.value))
		})
	}
}

func TestAnyToYAMLIndent(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		prefix string
		indent int
		want   string
	}{
		{name: "prefix and indent", value: map[string]any{"a": map[string]any{"b": 1}}, prefix: "> ", indent: 2, want: "> a:\n>   b: 1\n"},
		{name: "indent out of range defaults to 4", value: map[string]any{"a": map[string]any{"b": 1}}, indent: 100, want: "a:\n    b: 1\n"},
		{name: "zero indent defaults to 4", value: map[string]any{"a": []int{1}}, indent: 0, want: "a:\n    - 1\n"},
		{name: "scalar with prefix", value: "s", prefix: "  ", indent: 2, want: "  s\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, AnyToYAMLIndent(tt.value, tt.prefix, tt.indent))
		})
	}
}
