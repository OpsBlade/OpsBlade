// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidComparisonOperator(t *testing.T) {
	for _, op := range []ComparisonOperator{Equals, Not, Contains, MoreThan, LessThan, BeginsWith, After, Before, DaysOld, MinutesOld} {
		assert.True(t, IsValidComparisonOperator(op), string(op))
	}
	assert.True(t, IsValidComparisonOperator("EQUAL"))
	assert.False(t, IsValidComparisonOperator("bogus"))
	assert.False(t, IsValidComparisonOperator(""))
}

func TestComparisonOperatorToLower(t *testing.T) {
	op := ComparisonOperator("CONTAINS")
	assert.Equal(t, Contains, op.ToLower())
}

func selectTestDoc() map[string]any {
	return map[string]any{
		"count": 5,
		"f":     1.5,
		"b":     true,
		"s":     "Hello",
		"ts":    "2020-01-02T03:04:05Z",
		"list":  []any{map[string]any{"k": "a"}, map[string]any{"k": "b"}},
		"m":     map[string]any{"k": "v"},
		"nul":   nil,
	}
}

func TestApplySelectionCriteria(t *testing.T) {
	tests := []struct {
		name     string
		criteria []SelectCriteria
		want     bool
	}{
		{name: "no criteria selects", criteria: nil, want: true},
		{name: "float equal", criteria: []SelectCriteria{{Field: "f", Value: 1.5, Compare: Equals}}, want: true},
		{name: "number equal via float", criteria: []SelectCriteria{{Field: "count", Value: 5.0, Compare: Equals}}, want: true},
		{name: "number equal via int", criteria: []SelectCriteria{{Field: "count", Value: 5, Compare: Equals}}, want: true},
		{name: "number greater via int", criteria: []SelectCriteria{{Field: "count", Value: 3, Compare: MoreThan}}, want: true},
		{name: "number less via int64", criteria: []SelectCriteria{{Field: "count", Value: int64(6), Compare: LessThan}}, want: true},
		{name: "float compared with int", criteria: []SelectCriteria{{Field: "f", Value: 1, Compare: MoreThan}}, want: true},
		{name: "null field matches nothing", criteria: []SelectCriteria{{Field: "nul", Value: "a", Compare: Equals}}, want: false},
		{name: "null field not", criteria: []SelectCriteria{{Field: "nul", Value: "a", Compare: Not}}, want: false},
		{name: "bool equal", criteria: []SelectCriteria{{Field: "b", Value: true, Compare: Equals}}, want: true},
		{name: "string equal case insensitive", criteria: []SelectCriteria{{Field: "s", Value: "hello", Compare: Equals}}, want: true},
		{name: "operator case insensitive", criteria: []SelectCriteria{{Field: "s", Value: "hello", Compare: "EQUAL"}}, want: true},
		{name: "string contains", criteria: []SelectCriteria{{Field: "s", Value: "ELL", Compare: Contains}}, want: true},
		{name: "string begins", criteria: []SelectCriteria{{Field: "S", Value: "he", Compare: BeginsWith}}, want: true},
		{name: "string not", criteria: []SelectCriteria{{Field: "s", Value: "other", Compare: Not}}, want: true},
		{name: "string not fails on match", criteria: []SelectCriteria{{Field: "s", Value: "Hello", Compare: Not}}, want: false},
		{name: "string not case insensitive", criteria: []SelectCriteria{{Field: "s", Value: "hello", Compare: Not}}, want: false},
		{name: "wildcard list match", criteria: []SelectCriteria{{Field: "list.*.k", Value: "b", Compare: Equals}}, want: true},
		{name: "wildcard list no match", criteria: []SelectCriteria{{Field: "list.*.k", Value: "z", Compare: Equals}}, want: false},
		{name: "list without wildcard", criteria: []SelectCriteria{{Field: "list.k", Value: "b", Compare: Equals}}, want: true},
		{name: "nested map", criteria: []SelectCriteria{{Field: "m.k", Value: "v", Compare: Equals}}, want: true},
		{name: "wildcard on map is not a list", criteria: []SelectCriteria{{Field: "m.*.k", Value: "v", Compare: Equals}}, want: false},
		{name: "map compared as scalar", criteria: []SelectCriteria{{Field: "m", Value: "v", Compare: Equals}}, want: false},
		{name: "list compared as scalar", criteria: []SelectCriteria{{Field: "list", Value: "a", Compare: Equals}}, want: false},
		{name: "missing field", criteria: []SelectCriteria{{Field: "nope", Value: "a", Compare: Equals}}, want: false},
		{name: "path through scalar", criteria: []SelectCriteria{{Field: "count.x", Value: 1.0, Compare: Equals}}, want: false},
		{name: "days old", criteria: []SelectCriteria{{Field: "ts", Value: 1, Compare: DaysOld}}, want: true},
		{name: "minutes old string", criteria: []SelectCriteria{{Field: "ts", Value: "1", Compare: MinutesOld}}, want: true},
		{name: "after", criteria: []SelectCriteria{{Field: "ts", Value: "2019-01-01", Compare: After}}, want: true},
		{name: "before", criteria: []SelectCriteria{{Field: "ts", Value: "2021-01-01", Compare: Before}}, want: true},
		{name: "after with bad date", criteria: []SelectCriteria{{Field: "ts", Value: "bad", Compare: After}}, want: false},
		{
			name: "all criteria must match",
			criteria: []SelectCriteria{
				{Field: "s", Value: "hello", Compare: Equals},
				{Field: "b", Value: false, Compare: Equals},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplySelectionCriteria(selectTestDoc(), tt.criteria)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestApplySelectionCriteriaErrors(t *testing.T) {
	got, err := ApplySelectionCriteria(nil, nil)
	require.NoError(t, err)
	assert.False(t, got)

	got, err = ApplySelectionCriteria(selectTestDoc(), []SelectCriteria{{Field: "count", Value: 5, Compare: "bogus"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid comparison operator: bogus")
	assert.False(t, got)
}

func TestCheckCriteriaInDocumentInvalid(t *testing.T) {
	crit := SelectCriteria{Field: "a", Value: "b", Compare: Equals}
	assert.False(t, checkCriteriaInDocument(make(chan int), crit))
	assert.False(t, checkCriteriaInDocument("not an object", crit))
}

func TestMatchesCriteriaTyped(t *testing.T) {
	str := "b"
	var nilStr *string
	old := "2020-01-01"

	tests := []struct {
		name  string
		value any
		crit  SelectCriteria
		want  bool
	}{
		{name: "nil pointer", value: nilStr, crit: SelectCriteria{Value: "b", Compare: Equals}, want: false},
		{name: "nil value", value: nil, crit: SelectCriteria{Value: "b", Compare: Equals}, want: false},
		// The operator is case-insensitive for every value type, not only strings
		{name: "number uppercase operator", value: 5.0, crit: SelectCriteria{Value: 5, Compare: "EQUAL"}, want: true},
		{name: "number uppercase not", value: 5.0, crit: SelectCriteria{Value: 6, Compare: "NOT"}, want: true},
		{name: "bool uppercase operator", value: true, crit: SelectCriteria{Value: true, Compare: "Equal"}, want: true},
		{name: "string not ignores case", value: "B", crit: SelectCriteria{Value: "b", Compare: Not}, want: false},
		{name: "pointer dereferenced", value: &str, crit: SelectCriteria{Value: "b", Compare: Equals}, want: true},
		{name: "string greater", value: "b", crit: SelectCriteria{Value: "a", Compare: MoreThan}, want: true},
		{name: "string less", value: "b", crit: SelectCriteria{Value: "z", Compare: LessThan}, want: true},
		{name: "string unsupported operator", value: "b", crit: SelectCriteria{Value: "b", Compare: "bogus"}, want: false},
		{name: "int greater", value: 3, crit: SelectCriteria{Value: 2, Compare: MoreThan}, want: true},
		{name: "int less", value: 3, crit: SelectCriteria{Value: 4, Compare: LessThan}, want: true},
		{name: "int not", value: 3, crit: SelectCriteria{Value: 4, Compare: Not}, want: true},
		{name: "int equal", value: 3, crit: SelectCriteria{Value: 3, Compare: Equals}, want: true},
		{name: "int unsupported operator", value: 3, crit: SelectCriteria{Value: 4, Compare: Contains}, want: false},
		{name: "int wrong criteria type", value: 3, crit: SelectCriteria{Value: "3", Compare: Equals}, want: false},
		{name: "int vs float criteria", value: 3, crit: SelectCriteria{Value: 3.0, Compare: Equals}, want: true},
		{name: "int vs int64 criteria", value: 3, crit: SelectCriteria{Value: int64(2), Compare: MoreThan}, want: true},
		{name: "int64 vs int criteria", value: int64(3), crit: SelectCriteria{Value: 3, Compare: Equals}, want: true},
		{name: "int64 vs float criteria", value: int64(3), crit: SelectCriteria{Value: 4.0, Compare: LessThan}, want: true},
		{name: "float vs int criteria", value: 1.5, crit: SelectCriteria{Value: 1, Compare: MoreThan}, want: true},
		{name: "float vs int64 criteria", value: 1.5, crit: SelectCriteria{Value: int64(2), Compare: Not}, want: true},
		{name: "int64 greater", value: int64(3), crit: SelectCriteria{Value: int64(2), Compare: MoreThan}, want: true},
		{name: "int64 less", value: int64(3), crit: SelectCriteria{Value: int64(4), Compare: LessThan}, want: true},
		{name: "int64 not", value: int64(3), crit: SelectCriteria{Value: int64(4), Compare: Not}, want: true},
		{name: "int64 equal", value: int64(3), crit: SelectCriteria{Value: int64(3), Compare: Equals}, want: true},
		{name: "int64 unsupported operator", value: int64(3), crit: SelectCriteria{Value: int64(4), Compare: Contains}, want: false},
		{name: "int64 wrong criteria type", value: int64(3), crit: SelectCriteria{Value: "3", Compare: Equals}, want: false},
		{name: "float greater", value: 1.5, crit: SelectCriteria{Value: 1.0, Compare: MoreThan}, want: true},
		{name: "float less", value: 1.5, crit: SelectCriteria{Value: 2.0, Compare: LessThan}, want: true},
		{name: "float not", value: 1.5, crit: SelectCriteria{Value: 2.0, Compare: Not}, want: true},
		{name: "float unsupported operator", value: 1.5, crit: SelectCriteria{Value: 2.0, Compare: Contains}, want: false},
		{name: "float wrong criteria type", value: 1.5, crit: SelectCriteria{Value: "1.5", Compare: Equals}, want: false},
		{name: "bool not", value: true, crit: SelectCriteria{Value: false, Compare: Not}, want: true},
		{name: "bool unsupported operator", value: true, crit: SelectCriteria{Value: false, Compare: Contains}, want: false},
		{name: "bool wrong criteria type", value: true, crit: SelectCriteria{Value: "true", Compare: Equals}, want: false},
		{name: "unsupported value type", value: []any{"a"}, crit: SelectCriteria{Value: "a", Compare: Equals}, want: false},
		{name: "days old float", value: old, crit: SelectCriteria{Value: 1.0, Compare: DaysOld}, want: true},
		{name: "days old bad string", value: old, crit: SelectCriteria{Value: "x", Compare: DaysOld}, want: false},
		{name: "days old bad type", value: old, crit: SelectCriteria{Value: true, Compare: DaysOld}, want: false},
		{name: "days old bad date", value: "bad", crit: SelectCriteria{Value: 1, Compare: DaysOld}, want: false},
		{name: "minutes old int", value: old, crit: SelectCriteria{Value: 1, Compare: MinutesOld}, want: true},
		{name: "minutes old float", value: old, crit: SelectCriteria{Value: 1.0, Compare: MinutesOld}, want: true},
		{name: "minutes old bad string", value: old, crit: SelectCriteria{Value: "x", Compare: MinutesOld}, want: false},
		{name: "minutes old bad type", value: old, crit: SelectCriteria{Value: true, Compare: MinutesOld}, want: false},
		{name: "minutes old bad date", value: "bad", crit: SelectCriteria{Value: 1, Compare: MinutesOld}, want: false},
		{name: "before with bad date", value: "bad", crit: SelectCriteria{Value: "2020-01-01", Compare: Before}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchesCriteria(tt.value, tt.crit))
		})
	}
}

func TestMatchesCriteriaAgeInFuture(t *testing.T) {
	future := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	assert.False(t, matchesCriteria(future, SelectCriteria{Value: 1, Compare: DaysOld}))
	assert.False(t, matchesCriteria(future, SelectCriteria{Value: 1, Compare: MinutesOld}))
}

func TestParsePossibleDate(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{in: "2020-01-02T03:04:05.123Z", want: time.Date(2020, 1, 2, 3, 4, 5, 123000000, time.UTC)},
		{in: "2020-01-02T03:04:05Z", want: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)},
		{in: "2020-01-02T03:04:05", want: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)},
		{in: "2020-01-02", want: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)},
		{in: "20200102030405", want: time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)},
		{in: "20200102", want: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parsePossibleDate(tt.in)
			require.NoError(t, err)
			assert.True(t, tt.want.Equal(got), "got %s", got)
		})
	}

	_, err := parsePossibleDate("nope")
	assert.Error(t, err)
}

func TestTraverseMapEmptyKeys(t *testing.T) {
	assert.False(t, traverseMap(map[string]any{"a": "b"}, nil, SelectCriteria{Value: "b", Compare: Equals}))
}
