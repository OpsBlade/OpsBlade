// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package load

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context  shared.TaskContext `yaml:"context" json:"context"`
	FileName string             `yaml:"filename" json:"filename"`
	Fields   []string           `yaml:"fields" json:"fields"`
}

func init() {
	shared.RegisterTask("variables_load", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)

	file, err := os.Open(t.FileName)
	if err != nil {
		return t.Context.Error("failed to open file", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	v := make(map[string]any)
	decoder := json.NewDecoder(file)
	if err = decoder.Decode(&v); err != nil {
		return t.Context.Error("failed to deserialize variables", err)
	}

	if t.Context.Debug {
		shared.Dump(v)
	}

	// Loaded variables can be filtered, but only do so if at least one filter is present
	if len(t.Fields) > 0 {
		v = shared.SelectFields(v, t.Fields)
	}

	return t.Context.Result(true, fmt.Sprintf("Loaded variables from %s", t.FileName), v)
}
