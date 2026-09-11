// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package workflow

import (
	"fmt"
	"time"
)

// Outcome is the overall result of executing a workflow
type Outcome string

const (
	OutcomeCompleted Outcome = "completed" // every task succeeded
	OutcomeWarnings  Outcome = "warnings"  // completed, but one or more tasks with on_fail: warn failed
	OutcomeStopped   Outcome = "stopped"   // stopped cleanly by exit_if or on_fail: stop
	OutcomeFatal     Outcome = "fatal"     // aborted by a fatal failure
)

// TaskFailure identifies a task and what it reported
type TaskFailure struct {
	Sequence int    `json:"sequence"`
	Name     string `json:"name,omitempty"`
	Task     string `json:"task,omitempty"`
	Msg      string `json:"msg,omitempty"`
}

// String returns a one-line identifier for the task
func (f TaskFailure) String() string {
	if f.Name == "" {
		return fmt.Sprintf("task %d [%s]", f.Sequence, f.Task)
	}
	return fmt.Sprintf("task %d \"%s\" [%s]", f.Sequence, f.Name, f.Task)
}

// Result summarises a workflow execution
type Result struct {
	Outcome    Outcome       `json:"outcome"`
	Warnings   []TaskFailure `json:"warnings,omitempty"`   // tasks that failed with on_fail: warn
	Fatal      *TaskFailure  `json:"fatal,omitempty"`      // the task that aborted the workflow
	StoppedAt  *TaskFailure  `json:"stopped_at,omitempty"` // the task that stopped the workflow cleanly
	TasksRun   int           `json:"tasks_run"`            // tasks executed, including skipped entries
	TasksTotal int           `json:"tasks_total"`          // tasks in the workflow
	Started    time.Time     `json:"started"`
}

// ExitCode maps the outcome to a process exit code: 0 unless the workflow aborted
func (r Result) ExitCode() int {
	if r.Outcome == OutcomeFatal {
		return 1
	}
	return 0
}

// Summary returns the one-line closing message for the console
func (r Result) Summary() string {
	switch r.Outcome {
	case OutcomeWarnings:
		return fmt.Sprintf("Completed with %d warning(s). Exiting with code %d.", len(r.Warnings), r.ExitCode())
	case OutcomeStopped:
		if r.StoppedAt != nil {
			return fmt.Sprintf("Workflow stopped at %s: %s. Exiting with code %d.", r.StoppedAt.String(), r.StoppedAt.Msg, r.ExitCode())
		}
		return fmt.Sprintf("Workflow stopped. Exiting with code %d.", r.ExitCode())
	case OutcomeFatal:
		if r.Fatal != nil {
			return fmt.Sprintf("Terminating due to fatal error in %s. Exiting with code %d.", r.Fatal.String(), r.ExitCode())
		}
		return fmt.Sprintf("Terminating due to fatal error. Exiting with code %d.", r.ExitCode())
	default:
		return fmt.Sprintf("All tasks complete. Exiting with code %d.", r.ExitCode())
	}
}
