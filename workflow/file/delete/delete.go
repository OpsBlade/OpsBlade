// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package delete

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context  shared.TaskContext `yaml:"context" json:"context"`   // Task context
	FileName string             `yaml:"filename" json:"filename"` // Filename to save variables to
}

func init() {
	shared.RegisterTask("file_delete", func(context shared.TaskContext) shared.Task {
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

	if t.FileName == "" {
		return t.Context.Error("unable to delete, filename is empty", nil)
	}

	if t.Context.DryRun {
		return t.Context.Result(true, fmt.Sprintf("Dry run: would delete file: %s", t.FileName), nil)
	}

	err = os.Remove(t.FileName)
	if err != nil {
		return t.Context.Error("failed delete file", err)
	}

	return t.Context.Result(true, fmt.Sprintf("Deleted %s", t.FileName), nil)
}
