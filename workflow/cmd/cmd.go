// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cmd

import (
	"encoding/json"
	"github.com/OpsBlade/OpsBlade/shared"
	"os/exec"
)

type Task struct {
	Context     shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials `yaml:"credentials" json:"credentials"` // Allow override of credentials
	Cmd         string             `yaml:"cmd" json:"cmd"`                 // Subject of the message
	Args        []string           `yaml:"args" json:"args"`               // Body of the message
	NoFail      bool               `yaml:"no_fail" json:"no_fail"`         // Do not fail if the command returns a non-zero exit code
}

func init() {
	shared.RegisterTask("cmd_exec", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error
	data := make(map[string]any)

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	// Debug information
	if t.Context.Debug {
		shared.DumpTask(t)
	}

	if t.Context.DryRun {
		return t.Context.Result(true, "DryRun, command not executed", data)
	}

	// Execute command using os/exec
	cmd := exec.Command(t.Cmd, t.Args...)
	output, err := cmd.CombinedOutput()

	// Store output in data map
	outputStr := string(output)
	data["cmd"] = t.Cmd
	data["cmd_args"] = t.Args
	data["cmd_output"] = outputStr

	// Handle error
	if err != nil {
		if t.NoFail {
			return t.Context.Result(true, "Command executed with non-zero exit code (ignored because no_fail is set)", data)
		}
		return t.Context.Error("command execution failed", err)
	}

	return t.Context.Result(true, "Command executed successfully", data)
}
