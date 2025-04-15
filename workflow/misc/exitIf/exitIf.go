// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package exitIf

import (
	"encoding/json"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context shared.TaskContext      `yaml:"context" json:"context"` // Task context
	Env     string                  `yaml:"env" json:"env"`         // Optional file to load into the environment
	Filters []shared.Filter         `yaml:"filters" json:"filters"` // Filters to pass to AWS API
	Select  []shared.SelectCriteria `yaml:"select" json:"select"`   // Selection criteria to apply to the list of AMIs
	Fields  []string                `yaml:"fields" json:"fields"`   // List of fields to return as data
}

func init() {
	shared.RegisterTask("exit_if", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	// Get the variables
	variables := shared.GetVars()

	// Apply selection criteria
	selected, err := shared.ApplySelectionCriteria(variables, t.Select)
	if err != nil {
		return t.Context.Error("failed applying selection criteria", err)
	}

	data := make(map[string]any)
	data["exit_if_result"] = selected

	if selected {
		return t.Context.Result(
			false,
			"Exit condition met, returning false to terminate workflow",
			data)
	}

	return t.Context.Result(
		true,
		"Exit condition not met, returning true to continue workflow",
		data)
}
