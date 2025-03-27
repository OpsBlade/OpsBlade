// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// SelectFields attempts to extract a list of fields (including wildcards for slices) from an
// arbitrary structure. Case-insensitive matches are used. In some limited cases, fields may be
// flattened.
func SelectFields(s any, fields []string) map[string]any {

	// AWS types make it difficult to walk the structure.
	// Convert to map[string]any first
	var input map[string]any

	// In some cases the structure will contain types, such as from AWS, that this function doesn't recognize.
	// The easiest way to address this is to serialize to JSON and then deserialize back to a map. This results in a
	// pure map[string]any structure that can be more easily traversed.
	jsonBytes, err := json.Marshal(s)
	if err != nil {
		return map[string]any{}
	}

	if err = json.Unmarshal(jsonBytes, &input); err != nil {
		return map[string]any{}
	}

	// If no fields are specified, return the entire map
	if len(fields) < 1 {
		return input
	}

	// Conflicts can occur if a field and it's children are both requested. It's easier
	// to deconflict the fields here than try to handle it in the recursive function.
	// Note that the returned fields are all lower case.
	deconflicedFields := removeConflictingFields(fields)

	// Create an output map and start recursively collecting fields
	output := make(map[string]any)
	collectFields("", input, deconflicedFields, output)
	return output
}

// collectFields recursively collects fields. It handles primitive types, maps, slices, and structs
// When it encounters a slice/array, it will iterate over each element and collect fields from each.
// In some cases, fields may be flattened. For example, if a user requests a.*.b.c (typically because * is a list),
// b and c will be flattened as "b.c" in the output.
func collectFields(path string, val any, fields []string, output map[string]any) {
	v := reflect.ValueOf(val)
	if !v.IsValid() {
		return
	}

	// If current path matches one of the fields (case-insensitively), store this value
	if path != "" && containsFieldIgnoreCase(fields, path) {
		output[path] = val
	}

	// Check if we've reached a terminal/primitive value and if so, stop recursion
	//goland:noinspection GoSwitchMissingCasesForIotaConsts
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return
	}

	// Process complex types
	switch v.Kind() {
	case reflect.Map:

		// Regular map processing
		for _, key := range v.MapKeys() {
			keyStr := fmt.Sprint(key.Interface())
			value := v.MapIndex(key).Interface()

			// Direct key check
			if containsFieldIgnoreCase(fields, keyStr) {
				output[keyStr] = value
			}

			// Continue with path-based traversal
			newPath := joinPath(path, keyStr)
			collectFields(newPath, value, fields, output)
		}

		// Skip dot notation handling if path is empty
		if path == "" {
			break
		}

		// Handle dot notation for maps with keys included
		prefix := path + "."
		for _, field := range fields {
			if strings.HasPrefix(strings.ToLower(field), strings.ToLower(prefix)) {
				suffix := field[len(prefix):] // Extract part after dot

				results := make(map[string]interface{})
				for _, key := range v.MapKeys() {
					keyStr := fmt.Sprint(key.Interface())
					elemValue := v.MapIndex(key).Interface()
					subData := make(map[string]interface{})
					collectFields("", elemValue, []string{suffix}, subData)
					if value, exists := subData[suffix]; exists {
						results[keyStr] = value
					}
				}

				if len(results) > 0 {
					output[field] = results
				}
			}
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			// Skip unexported fields
			if !v.Field(i).CanInterface() {
				continue
			}
			fieldName := t.Field(i).Name
			newPath := joinPath(path, fieldName)
			collectFields(newPath, v.Field(i).Interface(), fields, output)
		}
	case reflect.Slice, reflect.Array:
		if len(fields) == 0 {
			return
		}

		// Look for wildcard match
		for _, field := range fields {
			if strings.HasPrefix(strings.ToLower(field), strings.ToLower(path)+".*") {

				// Build a list of other fields with the same prefix
				var tmpFields []string
				for _, f := range fields {
					if strings.HasPrefix(strings.ToLower(f), strings.ToLower(path)+".*") {
						tmpFields = append(tmpFields, removeFirstWildcard(f))
					}
				}

				// At this point, we have a wildcard match, but collect fields won't realize
				// it's a list, so we need to collect the fields here
				var tempList []map[string]any

				// Recurse into each element of the slice
				for i := 0; i < v.Len(); i++ {
					tmpMap := make(map[string]any)
					collectFields("", v.Index(i).Interface(), tmpFields, tmpMap)
					if len(tmpMap) > 0 {
						tempList = append(tempList, tmpMap)
					}
				}
				if len(tempList) > 0 {
					output[path] = tempList // MIGHT NEED TO HACK OFF AFTER THE FIRST WILDCARD
				}
				return
			}
		}

		// Default slice logic, index-based
		for i := 0; i < v.Len(); i++ {
			newPath := joinPath(path, fmt.Sprintf("%d", i))
			collectFields(newPath, v.Index(i).Interface(), fields, output)
		}

		// Skip dot notation handling if path is empty
		if path == "" {
			break
		}

		// Handle dot notation fields (like "ami_data.Name")
		prefix := path + "."
		for _, field := range fields {
			if strings.HasPrefix(strings.ToLower(field), strings.ToLower(prefix)) {
				suffix := field[len(prefix):] // Extract part after dot
				var results []interface{}
				for i := 0; i < v.Len(); i++ {
					elemValue := v.Index(i).Interface()
					subData := make(map[string]interface{})
					collectFields("", elemValue, []string{suffix}, subData)
					if value, exists := subData[suffix]; exists {
						results = append(results, value)
					}
				}

				if len(results) > 0 {
					output[field] = results
				}
			}
		}
	default:
	}
}

// containsFieldIgnoreCase checks if a field is in a list of fields, ignoring case
func containsFieldIgnoreCase(fields []string, candidate string) bool {
	lowerCandidate := strings.ToLower(candidate)
	for _, f := range fields {
		if strings.ToLower(f) == lowerCandidate {
			return true
		}
	}
	return false
}

// joinPath concatenates two strings with a period separator
func joinPath(base, elem string) string {
	if base == "" {
		return elem
	}
	return base + "." + elem
}

// removeFirstWildcard removes up to and including the first wildcard (*)
func removeFirstWildcard(s string) string {
	idx := strings.Index(s, ".*.")
	if idx == -1 {
		return s
	}
	return s[idx+3:]
}

// removeConflictingFields removes fields that are the children of other fields
// For example, if "ami" and "ami.name" are both specified, "ami.name" will be removed
func removeConflictingFields(fields []string) []string {
	lcSet := make(map[string]bool)
	for _, f := range fields {
		lcSet[strings.ToLower(f)] = true
	}

	// Iterate over the lower case set of fields, and for each determine
	// if it conflicts with any other field. If it does, remove it.
	for field := range lcSet {
		for _, f := range fields {

			// Convert the field to lower case and add a period
			check := strings.ToLower(f) + "."

			// If field begins with check, mark it as false
			if strings.HasPrefix(field, check) {
				lcSet[field] = false
				break
			}
		}
	}

	// Now iterate over the set and build a new list of fields
	var newFields []string
	for field, valid := range lcSet {
		if valid {
			newFields = append(newFields, field)
		}
	}
	return newFields
}
