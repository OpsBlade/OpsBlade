// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package workflow

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns what was written
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	require.NoError(t, w.Close())
	return <-done
}

func TestOptions(t *testing.T) {
	w := New(WithJSON(true), WithDebug(true), WithDryRun(true))
	assert.True(t, w.JSON)
	assert.True(t, w.Debug)
	assert.True(t, w.DryRun)
	assert.Empty(t, w.Tasks)
}

func TestAddTaskJSONAndYAML(t *testing.T) {
	w := New()
	require.NoError(t, w.AddTaskJSON([]byte(`{"task": "test_fake", "name": "j"}`)))
	require.NoError(t, w.AddTaskYAML([]byte("task: test_fake\nname: y\n")))
	require.Len(t, w.Tasks, 2)
	assert.Equal(t, "j", w.Tasks[0]["name"])
	assert.Equal(t, "y", w.Tasks[1]["name"])

	err := w.AddTaskJSON([]byte("{not json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deserialization failure")
	err = w.AddTaskYAML([]byte("task: [unterminated"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deserialization failure")
	assert.Len(t, w.Tasks, 2, "failed additions do not add tasks")
}

func TestDump(t *testing.T) {
	w := New(WithDryRun(true))
	w.AddTask(task("ok", nil))
	out := captureStdout(t, w.Dump)
	assert.Contains(t, out, "Global dryrun: true")
	assert.Contains(t, out, "Global debug: false")
	assert.Contains(t, out, "Task 1:")
	assert.Contains(t, out, `"task": "test_fake"`)
}

func TestLoad_Errors(t *testing.T) {
	w := New()
	err := w.Load(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read file")

	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("tasks: [unterminated"), 0o600))
	err = w.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deserialization error")
}

func TestLoad_Stdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdin.yaml")
	require.NoError(t, os.WriteFile(path, []byte("dryrun: true\ntasks:\n  - task: test_fake\n"), 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	orig := os.Stdin
	os.Stdin = f
	t.Cleanup(func() { os.Stdin = orig })

	w := New()
	require.NoError(t, w.Load(""))
	assert.Equal(t, defaultName, w.Name, "a workflow from stdin gets the default name")
	assert.True(t, w.DryRun)
	require.Len(t, w.Tasks, 1)
}

func TestExecute_TaskLookupFailures(t *testing.T) {
	cases := []struct {
		name    string
		task    map[string]any
		wantMsg string
	}{
		{"missing task type", map[string]any{"name": "no type"}, "Task type is missing or not a string"},
		{"unknown task", map[string]any{"name": "bogus", "task": "no_such_task"}, "Invalid task: no_such_task"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			r := run(t, rec, tc.task, task("ok", nil))
			assert.Equal(t, OutcomeFatal, r.Outcome)
			require.NotNil(t, r.Fatal)
			assert.Equal(t, 1, r.Fatal.Sequence)
			assert.Contains(t, r.Fatal.Msg, tc.wantMsg)
			assert.Equal(t, 1, r.TasksRun)
			assert.Empty(t, ran, "the following task must not run")
		})
	}
}

func TestExecute_Skip(t *testing.T) {
	rec := &recorder{}
	skipped := task("ok", nil)
	skipped["skip"] = true
	r := run(t, rec, skipped, task("ok", nil))
	assert.Equal(t, OutcomeCompleted, r.Outcome)
	assert.Equal(t, 2, r.TasksRun)
	assert.Equal(t, []int{2}, ran, "a skipped task does not execute")
	require.Len(t, rec.results, 2)
	assert.Equal(t, "skipped", rec.results[0].Outcome)
	assert.Equal(t, "task_skipped", rec.results[0].MessageType)
	assert.Equal(t, "Task skipped", rec.results[0].Msg)

	rec = &recorder{vetoAt: 1}
	r = run(t, rec, skipped, task("ok", nil))
	assert.Equal(t, OutcomeFatal, r.Outcome, "a callback veto on a skipped task is fatal")
	require.NotNil(t, r.Fatal)
	assert.Equal(t, "halted by callback", r.Fatal.Msg)
	assert.Empty(t, ran)
}

func TestExecute_ConsoleJSONOutput(t *testing.T) {
	ran = nil
	w := New(WithJSON(true))
	w.AddTask(task("ok", nil))
	var r Result
	out := captureStdout(t, func() { r = w.Execute() })
	assert.Equal(t, OutcomeCompleted, r.Outcome)
	assert.Contains(t, out, `"message_type": "task_start"`)
	assert.Contains(t, out, `"message_type": "task_stop"`)
	assert.Contains(t, out, `"outcome": "success"`)
}

func TestResult_SummaryWithoutFailureDetail(t *testing.T) {
	assert.Equal(t, "Workflow stopped. Exiting with code 0.", Result{Outcome: OutcomeStopped}.Summary())
	assert.Equal(t, "Terminating due to fatal error. Exiting with code 1.", Result{Outcome: OutcomeFatal}.Summary())
}
