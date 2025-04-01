// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package sleep

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context shared.TaskContext `yaml:"context" json:"context"`
	Sleep   int                `yaml:"sleep" json:"sleep"` // Sleep time in seconds
}

func init() {
	shared.RegisterTask("sleep", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)

	if t.Sleep < 1 {
		return t.Context.Error("Sleep time must be one second or greater", nil)
	}

	if t.Context.Debug {
		fmt.Printf("Sleeping for %d seconds...", t.Sleep)
	}

	time.Sleep(time.Duration(t.Sleep) * time.Second)
	return t.Context.Result(true, fmt.Sprintf("Slept for %d seconds", t.Sleep), nil)
}
