// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"fmt"
)

// TaskInfo is used to report the start of a task
type TaskInfo struct {
	MessageType  string         `json:"message_type"  yaml:"message_type"`  // Message type
	Sequence     int            `json:"sequence"      yaml:"sequence"`      // Task sequence number
	Name         string         `json:"name"          yaml:"name"`          // Task name
	Task         string         `json:"task"          yaml:"task"`          // Task type
	Msg          string         `json:"msg,omitempty" yaml:"msg,omitempty"` // Task message (used primarily for errors)
	Instructions map[string]any `json:"instructions"  yaml:"instructions"`  // Task data
	Debug        bool           `json:"debug"         yaml:"debug"`         // Debug flag
}

// serialize is a non-exported function that attempts to serialize the task result to a JSON string
func (ti *TaskInfo) serialize(prefix, indent string) string {
	var err error
	var data []byte

	// Serialize the task result
	data, err = json.MarshalIndent(ti, prefix, indent)
	if err != nil {
		// If serialization fails, return an error result
		msg := fmt.Sprintf("error serializing task info: %s", err.Error())
		errorResult := TaskInfo{
			Sequence:     ti.Sequence,
			Name:         ti.Name,
			Task:         ti.Task,
			Msg:          msg,
			Instructions: nil}
		data, err = json.Marshal(errorResult)
		if err != nil {
			// If serializing the error result fails, return an empty string
			data = []byte{}
		}
	}
	return string(data)
}

// Serialize the task result to a JSON string
func (ti *TaskInfo) Serialize() string {
	return ti.serialize("", "")
}

// SerializePretty serializes the task result to a pretty-printed JSON string
func (ti *TaskInfo) SerializePretty() string {
	return ti.serialize("", "  ")
}

// String attempts to return a human-readable string representation of the task result
func (ti *TaskInfo) String() string {
	var r string

	if ti.Name == "" {
		r = fmt.Sprintf("* Starting task %d: [%s]\n", ti.Sequence, ti.Task)
	} else {
		r = fmt.Sprintf("* Starting task %d: \"%s\" [%s]\n", ti.Sequence, ti.Name, ti.Task)
	}

	if ti.Debug {
		if ti.Instructions != nil {
			if len(ti.Instructions) > 0 {
				r += fmt.Sprintf("Instructions:\n")
				r += AnyToYAMLIndent(ti.Instructions, "  ", 2)
			} else {
				r += "Instructions: none"
			}
		}
	}

	// Remove any trailing newline characters
	r = TrimTrailingNewlines(r)
	return r
}
