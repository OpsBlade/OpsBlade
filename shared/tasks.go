// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"fmt"
)

type Task interface {
	Execute() TaskResult
}

type TaskContext struct {
	Env          string `json:"env,omitempty"`          // Task environment (overrides global)
	DryRun       bool   `json:"dryrun,omitempty"`       // Dry run mode
	Debug        bool   `json:"debug,omitempty"`        // Debug mode
	Name         string `json:"name,omitempty"`         // Task name
	Task         string `json:"task"`                   // Task type
	Sequence     int    `json:"sequence"`               // Task sequence number
	Instructions []byte `json:"instructions,omitempty"` // Task instructions
	ErrorMessage string `json:"error_message,omitempty"` // Custom error message to display on failure
}

var TaskRegistry = make(map[string]func(TaskContext) Task)

// RegisterTask registers a task constructor with the task registry
// A unique taskID is required for each task.
func RegisterTask(taskID string, constructor func(TaskContext) Task) {
	TaskRegistry[taskID] = constructor
}

func (c *TaskContext) String() string {
	return fmt.Sprintf("Task %d \"%s\" (%s)", c.Sequence, c.Name, c.Task)
}

// Result returns a task result enriched with information from the task context
func (c *TaskContext) Result(success bool, msg string, data any) TaskResult {
	dataMap := AnyToMapAny(data)
	return TaskResult{
		MessageType: "task_stop",
		Success:     success,
		Msg:         msg,
		Sequence:    c.Sequence,
		Name:        c.Name,
		Task:        c.Task,
		Data:        dataMap}
}

// Error returns a task result error enriched with information from the task context
func (c *TaskContext) Error(msg string, err error) TaskResult {
	var fullMsg string
	if err == nil {
		fullMsg = msg
	} else {
		fullMsg = fmt.Sprintf("%s: %s", msg, err.Error())
	}
	
	// Append custom error message if provided
	if c.ErrorMessage != "" {
		fullMsg = fmt.Sprintf("%s\n\n%s\n", fullMsg, c.ErrorMessage)
	}
	
	return c.Result(false, fullMsg, nil)
}
