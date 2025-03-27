// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package load

import (
	"encoding/json"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context shared.TaskContext `yaml:"context" json:"context"`
	Set     []Variable         `yaml:"set" json:"set"`
}

type Variable struct {
	Name  string `yaml:"name"`
	Value any    `yaml:"value"`
}

func init() {
	shared.RegisterTask("variables_set", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)

	var data = make(map[string]any)
	for _, v := range t.Set {
		shared.SetVar(v.Name, v.Value)
		data[v.Name] = v.Value
	}

	return t.Context.Result(true, "variables set", data)
}
