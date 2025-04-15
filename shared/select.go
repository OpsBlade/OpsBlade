// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type MatchType string

type ComparisonOperator string

//goland:noinspection GoUnusedConst,GoUnusedConst,GoUnusedConst,GoUnusedConst,GoUnusedConst
const (
	Equals     ComparisonOperator = "equal"
	Not        ComparisonOperator = "not"
	Contains   ComparisonOperator = "contains"
	MoreThan   ComparisonOperator = "greater"
	LessThan   ComparisonOperator = "less"
	BeginsWith ComparisonOperator = "begins"
	After      ComparisonOperator = "after"
	Before     ComparisonOperator = "before"
	DaysOld    ComparisonOperator = "days_old"
	MinutesOld ComparisonOperator = "minutes_old"
)

type SelectCriteria struct {
	Field   string             `yaml:"field" json:"field"`
	Value   any                `yaml:"value" json:"value"`
	Compare ComparisonOperator `yaml:"compare" json:"compare"`
}

// IsValidComparisonOperator checks if a comparison operator is valid
func IsValidComparisonOperator(op ComparisonOperator) bool {
	switch op.ToLower() {
	case Equals, Not, Contains, MoreThan, LessThan,
		BeginsWith, After, Before, DaysOld, MinutesOld:
		return true
	default:
		return false
	}
}

func (comp *ComparisonOperator) ToLower() ComparisonOperator {
	return ComparisonOperator(strings.ToLower(string(*comp)))
}

func ApplySelectionCriteria(document any, criteria []SelectCriteria) (bool, error) {
	if document == nil {
		return false, nil
	}

	if len(criteria) == 0 {
		// If there are no criteria, everything is selected
		return true, nil
	}

	for _, criterion := range criteria {

		// Validate the comparison operator
		if !IsValidComparisonOperator(criterion.Compare) {
			return false, fmt.Errorf("invalid comparison operator: %s", criterion.Compare)
		}
		if !checkCriteriaInDocument(document, criterion) {
			return false, nil
		}
	}
	return true, nil
}

func matchesCriteria(value any, criteria SelectCriteria) bool {
	v := reflect.ValueOf(value)

	// If it's a pointer, dereference it
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}

	// Extract interface{} again after dereferencing
	actualValue := v.Interface()

	switch typedVal := actualValue.(type) {
	case string:
		wanted, _ := criteria.Value.(string)
		switch criteria.Compare.ToLower() {
		case MinutesOld:
			var minutes int
			switch v := criteria.Value.(type) {
			case int:
				minutes = v
			case float64:
				minutes = int(v)
			case string:
				parsedDays, err := strconv.Atoi(v)
				if err != nil {
					return false
				}
				minutes = parsedDays
			default:
				return false
			}

			t, err := parsePossibleDate(typedVal)
			if err != nil {
				return false
			}
			cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
			return t.Before(cutoff)
		case DaysOld:
			var days int
			switch v := criteria.Value.(type) {
			case int:
				days = v
			case float64:
				days = int(v)
			case string:
				parsedDays, err := strconv.Atoi(v)
				if err != nil {
					return false
				}
				days = parsedDays
			default:
				return false
			}

			t, err := parsePossibleDate(typedVal)
			if err != nil {
				return false
			}
			cutoff := time.Now().AddDate(0, 0, -days)
			return t.Before(cutoff)
		case After:
			t1, err1 := parsePossibleDate(typedVal)
			t2, err2 := parsePossibleDate(wanted)
			if err1 != nil || err2 != nil {
				return false
			}
			return t1.After(t2)
		case Before:
			t1, err1 := parsePossibleDate(typedVal)
			t2, err2 := parsePossibleDate(wanted)
			if err1 != nil || err2 != nil {
				return false
			}
			return t1.Before(t2)
		case Equals:
			return strings.ToLower(typedVal) == strings.ToLower(wanted)
		case Contains:
			return strings.Contains(strings.ToLower(typedVal), strings.ToLower(wanted))
		case Not:
			return typedVal != wanted
		case MoreThan:
			return typedVal > wanted
		case LessThan:
			return typedVal < wanted
		case BeginsWith:
			return strings.HasPrefix(strings.ToLower(typedVal), strings.ToLower(wanted))
		default:
			return false
		}
	case int:
		wanted, ok := criteria.Value.(int)
		if !ok {
			return false
		}
		switch criteria.Compare {
		case Equals:
			return typedVal == wanted
		case Not:
			return typedVal != wanted
		case MoreThan:
			return typedVal > wanted
		case LessThan:
			return typedVal < wanted
		default:
			return false
		}
	case float64:
		wanted, ok := criteria.Value.(float64)
		if !ok {
			return false
		}
		switch criteria.Compare {
		case Equals:
			return typedVal == wanted
		case Not:
			return typedVal != wanted
		case MoreThan:
			return typedVal > wanted
		case LessThan:
			return typedVal < wanted
		default:
			return false
		}
	case int64:
		wanted, ok := criteria.Value.(int64)
		if !ok {
			return false
		}
		switch criteria.Compare {
		case Equals:
			return typedVal == wanted
		case Not:
			return typedVal != wanted
		case MoreThan:
			return typedVal > wanted
		case LessThan:
			return typedVal < wanted
		default:
			return false
		}
	case bool:
		wanted, ok := criteria.Value.(bool)
		if !ok {
			return false
		}
		switch criteria.Compare {
		case Equals:
			return typedVal == wanted
		case Not:
			return typedVal != wanted
		default:
			return false
		}
	}
	return false
}

func parsePossibleDate(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
		"20060102150405",
		"20060102",
	}
	var parsed time.Time
	var err error
	for _, layout := range layouts {
		parsed, err = time.Parse(layout, s)
		if err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, err
}

func checkCriteriaInDocument(document any, criteria SelectCriteria) bool {

	// Convert to map[string]any first
	var data map[string]any

	// Marshal to JSON then unmarshal to map to flatten the structure
	jsonBytes, err := json.Marshal(document)
	if err != nil {
		return false
	}

	if err = json.Unmarshal(jsonBytes, &data); err != nil {
		return false
	}

	// Now use dot notation to access the fields
	return checkCriteriaInMap(data, criteria)
}

func checkCriteriaInMap(data map[string]any, criteria SelectCriteria) bool {
	keys := strings.Split(criteria.Field, ".")
	return traverseMap(data, keys, criteria)
}

func traverseMap(data map[string]any, keys []string, criteria SelectCriteria) bool {
	if len(keys) == 0 {
		return matchesCriteria(data, criteria)
	}
	key := keys[0]
	restKeys := keys[1:]

	// Handle normal key lookup first
	value, exists := data[key]
	if !exists {
		// Try case-insensitive match
		for k, v := range data {
			if strings.EqualFold(k, key) {
				value = v
				exists = true
				break
			}
		}
	}

	if !exists {
		return false
	}

	// Now handle next part based on what we found
	if len(restKeys) == 0 {
		return matchesCriteria(value, criteria)
	}

	// Check if next key is wildcard
	if restKeys[0] == "*" {
		// We need a slice/array to use wildcard
		slice, ok := value.([]interface{})
		if !ok {
			return false
		}

		// Process each element with remaining keys
		for _, elem := range slice {
			if mapElem, ok := elem.(map[string]interface{}); ok {
				if traverseMap(mapElem, restKeys[1:], criteria) {
					return true
				}
			}
		}
		return false
	}

	// Normal traversal
	if nextMap, ok := value.(map[string]interface{}); ok {
		return traverseMap(nextMap, restKeys, criteria)
	}

	// Handle slice with non-wildcard next key
	if slice, ok := value.([]interface{}); ok {
		for _, elem := range slice {
			if mapElem, ok := elem.(map[string]interface{}); ok {
				if traverseMap(mapElem, restKeys, criteria) {
					return true
				}
			}
		}
	}

	return false
}
