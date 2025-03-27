// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package dump

import (
	"encoding/json"

	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context shared.TaskContext `yaml:"context" json:"context"` // Task context
	Fields  []string           `yaml:"fields" json:"fields"`   // Fields to return
}

func init() {
	shared.RegisterTask("variables_dump", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)
	vars := shared.SelectFields(shared.GetVars(), t.Fields)
	return t.Context.Result(true, "variables attached", vars)
}
