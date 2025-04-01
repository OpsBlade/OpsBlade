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

//goland:noinspection GoUnusedExportedFunction
func ProcessVars(v any) {
	val := reflect.ValueOf(v).Elem()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if !field.CanSet() {
			continue
		}

		// Handle interface fields
		if field.Kind() == reflect.Interface && !field.IsNil() {
			elem := field.Elem()
			//goland:noinspection GoSwitchMissingCasesForIotaConsts
			switch elem.Kind() {
			case reflect.String:
				replaced := replaceVarsInString(elem.Interface())
				field.Set(reflect.ValueOf(replaced))
				continue
			case reflect.Struct:
				ProcessVars(elem.Addr().Interface())
				continue
			case reflect.Map, reflect.Slice, reflect.Array, reflect.Ptr:
				// If elem is a pointer and not nil, recurse if it points to a struct
				if elem.Kind() == reflect.Ptr && !elem.IsNil() {
					if elem.Elem().Kind() == reflect.Struct {
						ProcessVars(elem.Elem().Addr().Interface())
					}
				}

				// If elem is a map with string keys, process it like other maps
				if elem.Kind() == reflect.Map && elem.Type().Key().Kind() == reflect.String {
					m := elem.Interface().(map[string]any)
					newMap := make(map[string]any)
					for k, v := range m {
						rv := reflect.ValueOf(v)
						if rv.Kind() == reflect.Map || rv.Kind() == reflect.Struct {
							ProcessVars(&v)
						}
						newMap[k] = replaceVarsInString(v)
					}
					field.Set(reflect.ValueOf(newMap))
				}

				// If elem is a slice or array, iterate and recurse for each item
				if elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array {
					for i := 0; i < elem.Len(); i++ {
						item := elem.Index(i)
						if item.Kind() == reflect.Struct {
							ProcessVars(item.Addr().Interface())
						} else if item.Kind() == reflect.Map && item.Type().Key().Kind() == reflect.String {
							vm := item.Interface().(map[string]any)
							newMap := make(map[string]any)
							for k, v := range vm {
								ProcessVars(&v)
								newMap[k] = replaceVarsInString(v)
							}
							item.Set(reflect.ValueOf(newMap))
						}
					}
				}
			}
		}

		if field.Kind() == reflect.Struct {
			ProcessVars(field.Addr().Interface())
		}

		// Pointer create
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			elem := field.Elem()
			if elem.Kind() == reflect.Struct {
				ProcessVars(elem.Addr().Interface())
			}
		}

		// Handle maps
		if field.Kind() == reflect.Map && field.Type().Key().Kind() == reflect.String {
			if field.Type().Elem().Kind() == reflect.String {
				originalMap := field.Interface().(map[string]string)
				newMap := make(map[string]string)
				for k, v := range originalMap {
					newMap[k] = replaceVarsInString(v)
				}
				field.Set(reflect.ValueOf(newMap))
			} else {
				mapVal := field.Interface().(map[string]any)
				newMap := make(map[string]any)
				for key, value := range mapVal {
					// If the value is a slice, recurse over each element
					rv := reflect.ValueOf(value)
					if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
						slice := make([]any, rv.Len())
						for idx := 0; idx < rv.Len(); idx++ {
							elem := rv.Index(idx).Interface()
							// Recurse if it's a map or struct
							if reflect.TypeOf(elem).Kind() == reflect.Map ||
								reflect.TypeOf(elem).Kind() == reflect.Struct {
								ProcessVars(&elem)
							}
							slice[idx] = replaceVarsInString(elem)
						}
						newMap[key] = slice
					} else if rv.Kind() == reflect.Map || rv.Kind() == reflect.Struct {
						ProcessVars(&value)
						newMap[key] = value
					} else {
						newMap[key] = replaceVarsInString(value)
					}
				}
				field.Set(reflect.ValueOf(newMap))
				continue
			}
		}

		// Handle slices/arrays
		if field.Kind() == reflect.Slice || field.Kind() == reflect.Array {
			for j := 0; j < field.Len(); j++ {
				elem := field.Index(j)
				if elem.Kind() == reflect.Map && elem.Type().Key().Kind() == reflect.String {
					fmt.Println("MAP IN SLICE", elem.String())
					mapVal := elem.Interface().(map[string]any)
					newMap := make(map[string]any)
					for key, value := range mapVal {
						newMap[key] = replaceVarsInString(value)
					}
					field.Index(j).Set(reflect.ValueOf(newMap))
				} else if elem.Kind() == reflect.Struct {
					ProcessVars(elem.Addr().Interface())
				}
			}
			continue
		}

		// Handle strings
		if field.Kind() == reflect.String {
			str := field.String()
			for {
				start := strings.Index(str, "{{")
				end := strings.Index(str, "}}")
				if start == -1 || end == -1 || start > end {
					break
				}
				varName := str[start+2 : end]
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
				str = str[:start] + replacement + str[end+2:]
			}
			field.SetString(str)
		}
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
