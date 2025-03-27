// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"encoding/json"
	"fmt"
)

// TaskResult is used to report on the result of a task
type TaskResult struct {
	MessageType string         `json:"message_type"`   // Message type
	Success     bool           `json:"success"`        // Task success status
	Msg         string         `json:"msg,omitempty"`  // Task message
	Sequence    int            `json:"sequence"`       // Task sequence number
	Name        string         `json:"name,omitempty"` // Task name
	Task        string         `json:"task,omitempty"` // Task type
	Data        map[string]any `json:"data,omitempty"` // Task data
	NoVars      bool           `json:"-" yaml:"-"`     // Do not set variables from this data
}

// serialize is a non-exported function that attempts to serialize the task result to a JSON string
func (tr *TaskResult) serialize(prefix, indent string) string {
	var err error
	var data []byte

	// Serialize the task result
	data, err = json.MarshalIndent(tr, prefix, indent)
	if err != nil {
		// If serialization fails, return an error result
		msg := fmt.Sprintf("error serializing task result: %s", err.Error())
		errorResult := TaskResult{
			Success:  false,
			Msg:      msg,
			Sequence: tr.Sequence,
			Name:     tr.Name,
			Task:     tr.Task,
			Data:     nil}
		data, err = json.Marshal(errorResult)
		if err != nil {
			// If serializing the error result fails, return an empty string
			data = []byte{}
		}
	}
	return string(data)
}

// Serialize the task result to a JSON string
func (tr *TaskResult) Serialize() string {
	return tr.serialize("", "")
}

// SerializePretty serializes the task result to a pretty-printed JSON string
func (tr *TaskResult) SerializePretty() string {
	return tr.serialize("", "  ")
}

// String attempts to return a human-readable string representation of the task result
func (tr *TaskResult) String() string {
	var r string

	if tr.Name == "" {
		r = fmt.Sprintf("* Completed task %d: [%s]\n", tr.Sequence, tr.Task)
	} else {
		r = fmt.Sprintf("* Completed task %d: \"%s\" [%s]\n", tr.Sequence, tr.Name, tr.Task)
	}
	r += fmt.Sprintf("Success: %t\n", tr.Success)
	r += fmt.Sprintf("Message: %s\n", tr.Msg)
	if tr.Data != nil {
		if len(tr.Data) > 0 {
			r += fmt.Sprintf("Data:\n")
			r += AnyToYAMLIndent(tr.Data, "  ", 2)
		} else {
			r += "Data: none"
		}
	}

	// Remove any trailing newline characters
	r = TrimTrailingNewlines(r)
	return r
}
