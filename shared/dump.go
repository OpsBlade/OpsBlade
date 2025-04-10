// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// DumpTask pretty-prints the passed task structure and drops the Context.Instructions field
func DumpTask(s any) {
	// Create a copy of the task with Context.Instructions removed
	taskCopy := copyTaskWithoutInstructions(s)

	// Encapsulate to be JSON parser friendly
	data := map[string]any{"message_type": "task_dump", "task_dump": taskCopy}
	fmt.Println(Dump(data) + "\n")
}

// copyTaskWithoutInstructions creates a deep copy of a task struct but with Context.Instructions set to nil
func copyTaskWithoutInstructions(s any) any {
	// If nil, return nil
	if s == nil {
		return nil
	}

	// Get the value of s
	val := reflect.ValueOf(s)

	// If it's not a pointer, return as is (can't be a task struct)
	if val.Kind() != reflect.Ptr {
		return s
	}

	// Dereference the pointer
	val = val.Elem()

	// If it's not a struct, return as is (can't be a task struct)
	if val.Kind() != reflect.Struct {
		return s
	}

	// Create a new struct of the same type
	newVal := reflect.New(val.Type())

	// Copy all fields from the original struct to the new one
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		newField := newVal.Elem().Field(i)

		// Check if this field is named "Context"
		if val.Type().Field(i).Name == "Context" && field.Kind() == reflect.Struct {
			// Copy the Context field but clear Instructions
			copyContextWithoutInstructions(field, newField)
		} else if newField.CanSet() {
			newField.Set(field)
		}
	}

	return newVal.Interface()
}

// copyContextWithoutInstructions copies a Context struct but sets Instructions to nil
func copyContextWithoutInstructions(src, dst reflect.Value) {
	// Copy all fields from src to dst
	for i := 0; i < src.NumField(); i++ {
		srcField := src.Field(i)
		dstField := dst.Field(i)

		// If this is the Instructions field, set it to empty
		if src.Type().Field(i).Name == "Instructions" {
			// Set to empty byte slice if it's a byte slice
			if srcField.Kind() == reflect.Slice && srcField.Type().Elem().Kind() == reflect.Uint8 {
				dstField.Set(reflect.ValueOf([]byte{}))
			}
		} else if dstField.CanSet() {
			dstField.Set(srcField)
		}
	}
}

// Dump pretty-prints the passed structure
func Dump(s any) string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf("Failed to serialize data: %s", err.Error())
	}
	return string(data)
}
