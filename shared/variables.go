// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

var Variables = make(map[string]any)

// GetVar returns the value of a variable
//
//goland:noinspection GoUnusedExportedFunction
func GetVar(name string) any {
	if value, ok := Variables[name]; ok {
		return value
	}
	return nil
}

//goland:noinspection GoUnusedExportedFunction
func GetVarString(name string) string {
	if value, ok := Variables[name]; ok {
		return AnyToString(value)
	}
	return ""
}

//goland:noinspection GoUnusedExportedFunction
func GetVarInt(name string) int {
	if value, ok := Variables[name]; ok {
		return AnyToInt(value)
	}
	return 0
}

//goland:noinspection GoUnusedExportedFunction
func GetVarInt64(name string) int64 {
	if value, ok := Variables[name]; ok {
		return AnyToInt64(value)
	}
	return 0
}

//goland:noinspection GoUnusedExportedFunction
func GetVarBool(name string) bool {
	if value, ok := Variables[name]; ok {
		return AnyToBool(value)
	}
	return false
}

//goland:noinspection GoUnusedExportedFunction
func GetVarMapString(name string) map[string]string {
	if value, ok := Variables[name]; ok {
		return AnyToMapString(value)
	}
	return make(map[string]string)
}

//goland:noinspection GoUnusedExportedFunction
func GetVarList(name string) []string {
	if value, ok := Variables[name]; ok {
		return AnyToList(value)
	}
	return nil
}

//goland:noinspection GoUnusedExportedFunction
func GetVars() map[string]any {
	return Variables
}

//goland:noinspection GoUnusedExportedFunction
func SetVar(name string, value any) {
	Variables[name] = value
}

// ProcessVars processes the variables in a struct, replacing any {{...}} placeholders with their values.
func ProcessVars(v any) {
	val := reflect.ValueOf(v).Elem()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.CanSet() {
			continue
		}
		processField(field)
	}
}

func processField(field reflect.Value) {
	//goland:noinspection GoSwitchMissingCasesForIotaConsts
	switch field.Kind() {
	case reflect.String:
		field.SetString(replaceVarsInString(field.String()))
	case reflect.Map:
		processMap(field)
	case reflect.Slice, reflect.Array:
		processSlice(field)
	case reflect.Struct:
		ProcessVars(field.Addr().Interface())
	case reflect.Ptr:
		if !field.IsNil() && field.Elem().Kind() == reflect.Struct {
			ProcessVars(field.Elem().Addr().Interface())
		}
	case reflect.Interface:
		if !field.IsNil() {
			field.Set(reflect.ValueOf(processValue(field.Interface())))
		}
	}
}

// processValue returns v with {{...}} placeholders resolved in any nested strings.
// The value is copied into an addressable location so that structs held in an
// interface can be processed. Non-string scalars are returned unchanged.
func processValue(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String, reflect.Map, reflect.Slice, reflect.Array, reflect.Struct, reflect.Ptr:
		addressable := reflect.New(rv.Type()).Elem()
		addressable.Set(rv)
		processField(addressable)
		return addressable.Interface()
	default:
		return v
	}
}

func processMap(field reflect.Value) {
	// Check if it's map[string]string or map[string]any
	if field.Type().Elem().Kind() == reflect.String {
		original := field.Interface().(map[string]string)
		newMap := make(map[string]string)
		for k, v := range original {
			newMap[k] = replaceVarsInString(v)
		}
		field.Set(reflect.ValueOf(newMap))
		return
	}
	original := field.Interface().(map[string]any)
	newMap := make(map[string]any)
	for k, v := range original {
		newMap[k] = processValue(v)
	}
	field.Set(reflect.ValueOf(newMap))
}

func processSlice(field reflect.Value) {

	// Handle slice of strings
	if field.Type().Elem().Kind() == reflect.String {
		for i := 0; i < field.Len(); i++ {
			strValue := field.Index(i).String()
			field.Index(i).SetString(replaceVarsInString(strValue))
		}
		return
	}

	// Handle structs, maps, etc
	for i := 0; i < field.Len(); i++ {
		processField(field.Index(i))
	}
}

// Utility function to replace \{\{...\}\} placeholders in strings
func replaceVarsInString(str any) string {
	s, ok := str.(string)
	if !ok {
		return AnyToString(str)
	}
	for {
		start := strings.Index(s, "{{")
		end := strings.Index(s, "}}")
		if start == -1 || end == -1 || start > end {
			break
		}
		varName := s[start+2 : end]
		var replacement string
		switch varName {
		case "date":
			replacement = time.Now().Format("20060102")
		case "datetime":
			replacement = time.Now().Format("20060102150405")
		case "epoch":
			replacement = fmt.Sprintf("%d", time.Now().Unix())
		default:
			if resolvedValue, ok := Variables[varName]; ok {
				replacement = AnyToString(resolvedValue)
			}
		}
		s = s[:start] + replacement + s[end+2:]
	}
	return s
}
