// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// DumpTask pretty-prints the passed task structure, extracting some data from the TaskContext field
// but suppressing the Context and Credentials fields to avoid leaking sensitive information
func DumpTask(s any) {
	v := reflect.ValueOf(s)

	// Ensure it's a struct or a pointer to a struct
	if v.Kind() == reflect.Ptr {
		v = v.Elem() // Dereference pointer
	}

	if v.Kind() != reflect.Struct {
		fmt.Println("Not a struct")
		return
	}

	// Get the field named "Context"
	field := v.FieldByName("Context")
	if !field.IsValid() {
		fmt.Println("No Context field")
		return
	}

	// Perform type assertion to TaskContext
	taskContext, ok := field.Interface().(TaskContext)
	if !ok {
		fmt.Println("Context field is not of type TaskContext")
		return
	}

	fmt.Printf("- Dumping %s\n", taskContext.String())
	fmt.Printf("Debug:  %t\n", taskContext.Debug)
	fmt.Printf("DryRun: %t\n", taskContext.DryRun)

	// Create a copy of the struct without the Context field
	//dumpData := reflect.New(v.Type()).Elem()
	dumpData := make(map[string]any)
	for i := 0; i < v.NumField(); i++ {
		fieldName := v.Type().Field(i).Name
		fieldValue := v.Field(i).Interface()
		if fieldName == "Context" || fieldName == "Credentials" {
			dumpData[fieldName] = "(field suppressed)"
		} else {
			dumpData[fieldName] = fieldValue
		}
	}

	fmt.Println(Dump(dumpData) + "\n")
}

// Dump pretty-prints the passed structure
func Dump(s any) string {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Sprintf("Failed to serialize data: %s", err.Error())
	}
	return string(data)
}
