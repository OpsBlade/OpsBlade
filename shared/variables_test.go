// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetVars replaces the package-level Variables map for the duration of a test
func resetVars(t *testing.T) {
	t.Helper()
	saved := Variables
	Variables = make(map[string]any)
	t.Cleanup(func() { Variables = saved })
}

func TestGetSetVar(t *testing.T) {
	resetVars(t)

	SetVar("s", "text")
	SetVar("i", 7)
	SetVar("i64", int64(8))
	SetVar("b", "true")
	SetVar("m", map[string]any{"k": 1})
	SetVar("l", []int{1, 2})

	assert.Equal(t, "text", GetVar("s"))
	assert.Nil(t, GetVar("missing"))

	assert.Equal(t, "text", GetVarString("s"))
	assert.Equal(t, "", GetVarString("missing"))

	assert.Equal(t, 7, GetVarInt("i"))
	assert.Equal(t, 0, GetVarInt("missing"))

	assert.Equal(t, int64(8), GetVarInt64("i64"))
	assert.Equal(t, int64(0), GetVarInt64("missing"))

	assert.True(t, GetVarBool("b"))
	assert.False(t, GetVarBool("missing"))

	assert.Equal(t, map[string]string{"k": "1"}, GetVarMapString("m"))
	assert.Equal(t, map[string]string{}, GetVarMapString("missing"))

	assert.Equal(t, []string{"1", "2"}, GetVarList("l"))
	assert.Nil(t, GetVarList("missing"))

	assert.Len(t, GetVars(), 6)
}

func TestReplaceVarsInString(t *testing.T) {
	resetVars(t)
	SetVar("name", "world")
	SetVar("n", 3)

	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "no placeholders", in: "plain", want: "plain"},
		{name: "single", in: "hello {{name}}", want: "hello world"},
		{name: "multiple", in: "{{name}}-{{n}}-{{name}}", want: "world-3-world"},
		{name: "unknown becomes empty", in: "a{{missing}}b", want: "ab"},
		{name: "unbalanced left", in: "a{{b", want: "a{{b"},
		{name: "unbalanced right", in: "a}}b", want: "a}}b"},
		{name: "close before open", in: "}}{{", want: "}}{{"},
		{name: "non-string converted", in: 42, want: "42"},
		{name: "nil", in: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, replaceVarsInString(tt.in))
		})
	}
}

func TestReplaceVarsInStringBuiltins(t *testing.T) {
	resetVars(t)

	before := time.Now().Unix()
	assert.Regexp(t, regexp.MustCompile(`^\d{8}$`), replaceVarsInString("{{date}}"))
	assert.Regexp(t, regexp.MustCompile(`^\d{14}$`), replaceVarsInString("{{datetime}}"))

	epoch, err := strconv.ParseInt(replaceVarsInString("{{epoch}}"), 10, 64)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, epoch, before)
}

func TestProcessVars(t *testing.T) {
	resetVars(t)
	SetVar("x", "X")

	type inner struct {
		S string
	}
	type target struct {
		Str      string
		Strs     []string
		Arr      [2]string
		MapStr   map[string]string
		MapAny   map[string]any
		Struct   inner
		Ptr      *inner
		NilPtr   *inner
		Iface    any
		NilIface any
		Structs  []inner
		Num      int
		hidden   string
	}

	in := target{
		Str:     "a{{x}}",
		Strs:    []string{"{{x}}", "b"},
		Arr:     [2]string{"{{x}}", "c"},
		MapStr:  map[string]string{"k": "{{x}}"},
		MapAny:  map[string]any{"k": "{{x}}", "l": []any{"{{x}}"}},
		Struct:  inner{S: "{{x}}"},
		Ptr:     &inner{S: "{{x}}"},
		Iface:   "{{x}}",
		Structs: []inner{{S: "{{x}}"}},
		Num:     1,
		hidden:  "{{x}}",
	}

	ProcessVars(&in)

	assert.Equal(t, "aX", in.Str)
	assert.Equal(t, []string{"X", "b"}, in.Strs)
	assert.Equal(t, [2]string{"X", "c"}, in.Arr)
	assert.Equal(t, map[string]string{"k": "X"}, in.MapStr)
	assert.Equal(t, map[string]any{"k": "X", "l": []any{"X"}}, in.MapAny)
	assert.Equal(t, "X", in.Struct.S)
	assert.Equal(t, "X", in.Ptr.S)
	assert.Nil(t, in.NilPtr)
	assert.Equal(t, "X", in.Iface)
	assert.Nil(t, in.NilIface)
	assert.Equal(t, []inner{{S: "X"}}, in.Structs)
	assert.Equal(t, 1, in.Num)
	assert.Equal(t, "{{x}}", in.hidden, "unexported fields are left alone")
}
