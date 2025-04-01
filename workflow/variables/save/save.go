// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package save

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	FileName string             `yaml:"filename" json:"filename"` // Filename to save variables to
	Context  shared.TaskContext `yaml:"context" json:"context"`   // Task context
	Fields   []string           `yaml:"fields" json:"fields"`     // Fields to return
}

func init() {
	shared.RegisterTask("variables_save", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Process variables, it is possible that our filename is one
	shared.ProcessVars(t)

	// Get all variables
	vars := shared.GetVars()

	// Apply field selection
	selectedVars := shared.SelectFields(vars, t.Fields)

	if t.Context.Debug {
		fmt.Println("Selected variables to save:\n", shared.Dump(selectedVars))
	}

	if t.Context.DryRun {
		return t.Context.Result(true, fmt.Sprintf("Dry run: would save variables to %s", t.FileName), nil)
	}

	file, err := os.Create(t.FileName)
	if err != nil {
		return t.Context.Error("failed to create file", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	encoder := json.NewEncoder(file)
	if err = encoder.Encode(selectedVars); err != nil {
		return t.Context.Error("failed to serialize variables", err)
	}

	r := t.Context.Result(true, fmt.Sprintf("Saved variables to %s", t.FileName), shared.SelectFields(vars, t.Fields))

	// Do not save variables from this data
	r.NoVars = true
	return r
}
