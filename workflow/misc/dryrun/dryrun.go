// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package dryrun

import (
	"errors"

	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context shared.TaskContext `yaml:"context" json:"context"`
}

func init() {
	shared.RegisterTask("dryrun_or_die", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	if t.Context.DryRun {
		return t.Context.Result(true, "Dryrun is confirmed", nil)
	}
	return t.Context.Error("Dryrun is required but not enabled, returning error", errors.New("dryrun required but not enabled"))
}
