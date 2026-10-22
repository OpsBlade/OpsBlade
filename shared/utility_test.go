// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrimTrailingNewlines(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "a\n\n", want: "a"},
		{in: "a", want: "a"},
		{in: "", want: ""},
		{in: "\n", want: ""},
		{in: "a\nb\n", want: "a\nb"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, TrimTrailingNewlines(tt.in))
		})
	}
}

func TestOneField(t *testing.T) {
	type fields struct {
		S string
		I int
		B bool
		F float64
		U uint
		P *int
		L []string
		M map[string]string
	}
	one := 1

	tests := []struct {
		name string
		in   fields
		want bool
	}{
		{name: "all zero", in: fields{}, want: false},
		{name: "string", in: fields{S: "x"}, want: true},
		{name: "int", in: fields{I: 1}, want: true},
		{name: "bool", in: fields{B: true}, want: true},
		{name: "float", in: fields{F: 0.5}, want: true},
		{name: "uint", in: fields{U: 1}, want: true},
		{name: "pointer", in: fields{P: &one}, want: true},
		{name: "slice", in: fields{L: []string{"a"}}, want: true},
		{name: "map", in: fields{M: map[string]string{"a": "b"}}, want: true},
		{name: "empty slice", in: fields{L: []string{}}, want: false},
		{name: "empty map", in: fields{M: map[string]string{}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, OneField(tt.in))
		})
	}
}
