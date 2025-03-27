// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import "reflect"

func TrimTrailingNewlines(r string) string {
	for len(r) > 0 && r[len(r)-1] == '\n' {
		r = r[:len(r)-1]
	}
	return r
}

// OneField accepts any structure and returns true if at least one field is set
// to a non-zero, non-empty, or true value.
func OneField(s any) bool {
	v := reflect.ValueOf(s)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		switch field.Kind() {
		case reflect.String:
			if field.String() != "" {
				return true
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Int() != 0 {
				return true
			}
		case reflect.Bool:
			if field.Bool() {
				return true
			}
		case reflect.Float32, reflect.Float64:
			if field.Float() != 0 {
				return true
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if field.Uint() != 0 {
				return true
			}
		default:
			if !field.IsZero() {
				return true
			}
		}
	}
	return false
}
