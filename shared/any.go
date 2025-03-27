// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"reflect"
	"strconv"
	"strings"
)

func AnyToString(value any) string {
	if value == nil {
		return ""
	}

	rv := reflect.ValueOf(value)

	// Handle slices and arrays
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		strs := make([]string, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			strs[i] = fmt.Sprintf("%v", rv.Index(i).Interface())
		}
		return strings.Join(strs, ",")
	}

	// Handle maps by converting to a comma-delimited string of "key:value" pairs
	if rv.Kind() == reflect.Map {
		keys := rv.MapKeys()
		strs := make([]string, len(keys))
		for i, key := range keys {
			keyStr := fmt.Sprintf("%v", key.Interface())
			valueStr := fmt.Sprintf("%v", rv.MapIndex(key).Interface())
			strs[i] = fmt.Sprintf("%s:%s", keyStr, valueStr)
		}
		return strings.Join(strs, ",")
	}

	// Handle primitive types
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func AnyToInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	default:
		return 0
	}
}

func AnyToInt64(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		i, _ := strconv.ParseInt(v, 10, 64)
		return i
	default:
		return 0
	}
}

func AnyToBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	case int:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func AnyToMapString(value any) map[string]string {
	if value == nil {
		return map[string]string{}
	}

	result := make(map[string]string)
	rv := reflect.ValueOf(value)

	// If it's a slice or array, convert each element into a map entry
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		for i := 0; i < rv.Len(); i++ {
			result[strconv.Itoa(i)] = fmt.Sprintf("%v", rv.Index(i).Interface())
		}
		return result
	}

	switch m := value.(type) {
	case map[string]string:
		return m
	case map[string]any:
		for k, v := range m {
			result[k] = fmt.Sprintf("%v", v)
		}
	case map[string]int:
		for k, v := range m {
			result[k] = fmt.Sprintf("%d", v)
		}
	case map[string]int64:
		for k, v := range m {
			result[k] = fmt.Sprintf("%v", v)
		}
	case map[string]bool:
		for k, v := range m {
			result[k] = fmt.Sprintf("%t", v)
		}
	case string:
		result["0"] = value.(string)
	case int:
		result["0"] = strconv.Itoa(value.(int))
	case int64:
		result["0"] = strconv.FormatInt(value.(int64), 10)
	case float64:
		result["0"] = strconv.FormatFloat(value.(float64), 'f', -1, 64)
	case bool:
		result["0"] = strconv.FormatBool(value.(bool))
	}
	return result
}

func AnyToMapAny(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}

	result := make(map[string]any)
	rv := reflect.ValueOf(value)

	// Handle nil or invalid values
	if !rv.IsValid() {
		return result
	}

	switch rv.Kind() {
	case reflect.Map:
		// Direct return if already map[string]any
		if m, ok := value.(map[string]any); ok {
			return m
		}

		// Convert other map types recursively
		for _, key := range rv.MapKeys() {
			keyStr := fmt.Sprint(key.Interface())
			mapValue := rv.MapIndex(key).Interface()

			// Recursively handle nested values
			switch reflect.ValueOf(mapValue).Kind() {
			case reflect.Map, reflect.Struct:
				result[keyStr] = AnyToMapAny(mapValue)
			case reflect.Slice, reflect.Array:
				result[keyStr] = convertSliceToAny(mapValue)
			default:
				result[keyStr] = mapValue
			}
		}

	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			result[strconv.Itoa(i)] = convertToAny(rv.Index(i).Interface())
		}

	case reflect.Struct:
		t := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			if rv.Field(i).CanInterface() {
				fieldName := t.Field(i).Name
				result[fieldName] = convertToAny(rv.Field(i).Interface())
			}
		}

	default:
		result["0"] = value
	}

	return result
}

func convertSliceToAny(slice any) []any {
	rv := reflect.ValueOf(slice)
	result := make([]any, rv.Len())

	for i := 0; i < rv.Len(); i++ {
		result[i] = convertToAny(rv.Index(i).Interface())
	}

	return result
}

func convertToAny(value any) any {
	rv := reflect.ValueOf(value)

	if !rv.IsValid() {
		return nil
	}

	switch rv.Kind() {
	case reflect.Map, reflect.Struct:
		return AnyToMapAny(value)
	case reflect.Slice, reflect.Array:
		return convertSliceToAny(value)
	default:
		return value
	}
}

func AnyToList(value any) []string {
	// Handle nil case
	if value == nil {
		return nil
	}

	rv := reflect.ValueOf(value)

	// Handle slices and arrays
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		list := make([]string, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			list[i] = fmt.Sprintf("%v", rv.Index(i).Interface())
		}
		return list
	}

	// Handle maps by converting to a list of "key:value" strings
	if rv.Kind() == reflect.Map {
		keys := rv.MapKeys()
		list := make([]string, len(keys))
		for i, key := range keys {
			keyStr := fmt.Sprintf("%v", key.Interface())
			valueStr := fmt.Sprintf("%v", rv.MapIndex(key).Interface())
			list[i] = fmt.Sprintf("%s:%s", keyStr, valueStr)
		}
		return list
	}

	// Handle primitive types by returning a single-item list
	return []string{AnyToString(value)}
}

// AnyToYAML converts any value to a YAML string
// The go yaml package is more restrictive than the JSON package, so this function
// first serializes to JSON, which ignores non-exported fields, and then deserializes
// to YAML
//
//goland:noinspection GoUnusedExportedFunction
func AnyToYAML(value any) string {
	return anyToYAML(value, "", 0)
}

// AnyToYAMLIndent converts any value to a YAML string with the specified prefix and indent
func AnyToYAMLIndent(value any, prefix string, indent int) string {
	return anyToYAML(value, prefix, indent)
}

func anyToYAML(value any, prefix string, indent int) string {
	var err error

	// Default to 4 spaces
	if indent < 1 || indent > 64 {
		indent = 4
	}

	// Marshal original data into JSON
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("unable to serialize to JSON: %s", err.Error())
	}

	// Unmarshal JSON into a map
	var doc map[string]any
	if err = json.Unmarshal(jsonBytes, &doc); err != nil {
		return fmt.Sprintf("unable to deserialize to a map: %s", err.Error())
	}

	// Use an encoder with custom indentation
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indent) // Set 4 spaces
	_ = enc.Encode(doc)

	// Add prefix if required
	var result string
	if prefix != "" {
		lines := strings.Split(buf.String(), "\n")
		for i, line := range lines {
			if line != "" {
				lines[i] = prefix + line
			}
		}
		result = strings.Join(lines, "\n")
	} else {
		result = buf.String()
	}

	return result
}
