// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/notify"
	"github.com/OpsBlade/OpsBlade/shared"
)

// fakeTask is a test-only task controlled by its "mode" instruction:
// succeed, fail, or stop. It records the sequence of every task that runs.
type fakeTask struct {
	Context shared.TaskContext `json:"context"`
	Mode    string             `json:"mode"`
}

var ran []int

func init() {
	shared.RegisterTask("test_fake", func(context shared.TaskContext) shared.Task {
		return &fakeTask{Context: context}
	})
}

func (t *fakeTask) Execute() shared.TaskResult {
	if err := json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}
	ran = append(ran, t.Context.Sequence)
	switch t.Mode {
	case "fail":
		return t.Context.Error("simulated failure", nil)
	case "stop":
		return t.Context.Stop("simulated stop", nil)
	case "fail_override_warn", "fail_override_stop", "fail_override_fatal":
		r := t.Context.Error("simulated expected condition", nil)
		r.OnFail = strings.TrimPrefix(t.Mode, "fail_override_")
		return r
	default:
		return t.Context.Result(true, "ok", map[string]any{"fake_ran": t.Context.Sequence})
	}
}

// recorder is a quiet callback that records results and can veto a given sequence
type recorder struct {
	results []shared.TaskResult
	vetoAt  int
}

func (r *recorder) OnStart(shared.TaskInfo) bool { return true }
func (r *recorder) OnStop(result shared.TaskResult) bool {
	r.results = append(r.results, result)
	return r.vetoAt == 0 || result.Sequence != r.vetoAt
}

func task(mode string, onFail any) map[string]any {
	m := map[string]any{"name": mode + " task", "task": "test_fake", "mode": mode}
	if onFail != nil {
		m["on_fail"] = onFail
	}
	return m
}

func run(t *testing.T, rec *recorder, tasks ...map[string]any) Result {
	t.Helper()
	ran = nil
	w := New(WithCallback(rec))
	for _, task := range tasks {
		w.AddTask(task)
	}
	return w.Execute()
}

func TestExecute_OnFail(t *testing.T) {
	cases := []struct {
		name        string
		tasks       []map[string]any
		wantOutcome Outcome
		wantRan     []int
		wantExit    int
		wantWarn    int
		wantFatalAt int
		wantStopAt  int
	}{
		{
			name:        "all succeed",
			tasks:       []map[string]any{task("succeed", nil), task("succeed", nil)},
			wantOutcome: OutcomeCompleted,
			wantRan:     []int{1, 2},
		},
		{
			name:        "default is fatal",
			tasks:       []map[string]any{task("fail", nil), task("succeed", nil)},
			wantOutcome: OutcomeFatal,
			wantRan:     []int{1},
			wantExit:    1,
			wantFatalAt: 1,
		},
		{
			name:        "explicit fatal",
			tasks:       []map[string]any{task("fail", "fatal"), task("succeed", nil)},
			wantOutcome: OutcomeFatal,
			wantRan:     []int{1},
			wantExit:    1,
			wantFatalAt: 1,
		},
		{
			name:        "warn continues and records",
			tasks:       []map[string]any{task("fail", "warn"), task("succeed", nil), task("fail", "warn")},
			wantOutcome: OutcomeWarnings,
			wantRan:     []int{1, 2, 3},
			wantWarn:    2,
		},
		{
			name:        "stop from failure is clean",
			tasks:       []map[string]any{task("fail", "stop"), task("succeed", nil)},
			wantOutcome: OutcomeStopped,
			wantRan:     []int{1},
			wantStopAt:  1,
		},
		{
			name:        "stop requested by task",
			tasks:       []map[string]any{task("succeed", nil), task("stop", nil), task("succeed", nil)},
			wantOutcome: OutcomeStopped,
			wantRan:     []int{1, 2},
			wantStopAt:  2,
		},
		{
			name:        "stop requested by task overrides warn",
			tasks:       []map[string]any{task("stop", "warn"), task("succeed", nil)},
			wantOutcome: OutcomeStopped,
			wantRan:     []int{1},
			wantStopAt:  1,
		},
		{
			name:        "warning then fatal keeps both",
			tasks:       []map[string]any{task("fail", "warn"), task("fail", nil), task("succeed", nil)},
			wantOutcome: OutcomeFatal,
			wantRan:     []int{1, 2},
			wantExit:    1,
			wantWarn:    1,
			wantFatalAt: 2,
		},
		{
			name:        "invalid on_fail aborts before any task runs",
			tasks:       []map[string]any{task("succeed", nil), task("succeed", "continue")},
			wantOutcome: OutcomeFatal,
			wantRan:     nil,
			wantExit:    1,
			wantFatalAt: 2,
		},
		{
			name:        "non-string on_fail aborts before any task runs",
			tasks:       []map[string]any{task("succeed", true)},
			wantOutcome: OutcomeFatal,
			wantRan:     nil,
			wantExit:    1,
			wantFatalAt: 1,
		},
		{
			name:        "unknown task type is fatal by default",
			tasks:       []map[string]any{{"task": "no_such_task"}, task("succeed", nil)},
			wantOutcome: OutcomeFatal,
			wantRan:     nil,
			wantExit:    1,
			wantFatalAt: 1,
		},
		{
			name:        "unknown task type with warn continues",
			tasks:       []map[string]any{{"task": "no_such_task", "on_fail": "warn"}, task("succeed", nil)},
			wantOutcome: OutcomeWarnings,
			wantRan:     []int{2},
			wantWarn:    1,
		},
		{
			name:        "missing task type with stop",
			tasks:       []map[string]any{{"name": "no type", "on_fail": "stop"}, task("succeed", nil)},
			wantOutcome: OutcomeStopped,
			wantRan:     nil,
			wantStopAt:  1,
		},
		{
			name:        "skipped task does not run",
			tasks:       []map[string]any{{"task": "test_fake", "mode": "fail", "skip": true}, task("succeed", nil)},
			wantOutcome: OutcomeCompleted,
			wantRan:     []int{2},
		},
		{
			name:        "empty workflow completes",
			tasks:       nil,
			wantOutcome: OutcomeCompleted,
			wantRan:     nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			r := run(t, rec, tc.tasks...)

			assert.Equal(t, tc.wantOutcome, r.Outcome)
			assert.Equal(t, tc.wantRan, ran, "tasks that executed")
			assert.Equal(t, tc.wantExit, r.ExitCode())
			assert.Len(t, r.Warnings, tc.wantWarn)
			assert.Equal(t, len(tc.tasks), r.TasksTotal)
			if tc.wantFatalAt != 0 {
				require.NotNil(t, r.Fatal)
				assert.Equal(t, tc.wantFatalAt, r.Fatal.Sequence)
			} else {
				assert.Nil(t, r.Fatal)
			}
			if tc.wantStopAt != 0 {
				require.NotNil(t, r.StoppedAt)
				assert.Equal(t, tc.wantStopAt, r.StoppedAt.Sequence)
			} else {
				assert.Nil(t, r.StoppedAt)
			}
			assert.False(t, r.Started.IsZero())
		})
	}
}

// A result that carries its own on_fail is classified by that value, not the
// task's on_fail. This is how a task separates an expected condition from an error.
func TestExecute_ResultOnFailOverride(t *testing.T) {
	cases := []struct {
		name        string
		mode        string
		onFail      any
		wantOutcome Outcome
		wantRan     []int
		wantMsg     string
	}{
		{"override warn beats fatal", "fail_override_warn", nil, OutcomeWarnings, []int{1, 2}, "simulated expected condition (continuing: warn)"},
		{"override stop beats fatal", "fail_override_stop", nil, OutcomeStopped, []int{1}, "simulated expected condition"},
		{"override fatal beats warn", "fail_override_fatal", "warn", OutcomeFatal, []int{1}, "simulated expected condition"},
		{"override stop beats warn", "fail_override_stop", "warn", OutcomeStopped, []int{1}, "simulated expected condition"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			r := run(t, rec, task(tc.mode, tc.onFail), task("succeed", nil))
			assert.Equal(t, tc.wantOutcome, r.Outcome)
			assert.Equal(t, tc.wantRan, ran)
			assert.Equal(t, tc.wantMsg, rec.results[0].Msg)
			if tc.wantOutcome == OutcomeStopped {
				require.NotNil(t, r.StoppedAt)
				assert.Equal(t, 1, r.StoppedAt.Sequence)
				assert.Nil(t, r.Fatal)
			}
			if tc.wantOutcome == OutcomeFatal {
				require.NotNil(t, r.Fatal)
				assert.Equal(t, 1, r.Fatal.Sequence)
			}
		})
	}
}

func TestExecute_ResultOutcomeLabels(t *testing.T) {
	rec := &recorder{}
	run(t, rec,
		task("succeed", nil),
		task("fail", "warn"),
		map[string]any{"task": "test_fake", "skip": true},
		task("stop", nil),
	)
	require.Len(t, rec.results, 4)
	assert.Equal(t, "success", rec.results[0].Outcome)
	assert.Equal(t, "warning", rec.results[1].Outcome)
	assert.False(t, rec.results[1].Success)
	assert.Contains(t, rec.results[1].Msg, "on_fail is warn")
	assert.Equal(t, "skipped", rec.results[2].Outcome)
	assert.Equal(t, "stop", rec.results[3].Outcome)
	assert.True(t, rec.results[3].Success)
	assert.True(t, rec.results[3].Stop)
}

func TestExecute_CallbackVetoIsFatal(t *testing.T) {
	rec := &recorder{vetoAt: 2}
	r := run(t, rec, task("succeed", nil), task("succeed", nil), task("succeed", nil))

	assert.Equal(t, OutcomeFatal, r.Outcome)
	assert.Equal(t, []int{1, 2}, ran)
	require.NotNil(t, r.Fatal)
	assert.Equal(t, 2, r.Fatal.Sequence)
	assert.Equal(t, "halted by callback", r.Fatal.Msg)
	assert.Equal(t, 1, r.ExitCode())
}

func TestExecute_CallbackVetoDoesNotOverrideStop(t *testing.T) {
	rec := &recorder{vetoAt: 1}
	r := run(t, rec, task("stop", nil), task("succeed", nil))

	assert.Equal(t, OutcomeStopped, r.Outcome)
	assert.Nil(t, r.Fatal)
	assert.Equal(t, 0, r.ExitCode())
}

func TestExecute_ConsoleOutputPathContinuesOnWarn(t *testing.T) {
	// No callback: results are printed. Continuation must still be decided by on_fail.
	ran = nil
	w := New()
	w.AddTask(task("fail", "warn"))
	w.AddTask(task("succeed", nil))
	r := w.Execute()

	assert.Equal(t, OutcomeWarnings, r.Outcome)
	assert.Equal(t, []int{1, 2}, ran)
}

func TestExecute_VariablesSetFromResults(t *testing.T) {
	rec := &recorder{}
	run(t, rec, task("succeed", nil))
	assert.Equal(t, 1, shared.GetVar("fake_ran"))
}

func TestParseOnFail(t *testing.T) {
	cases := []struct {
		raw     map[string]any
		want    string
		wantErr bool
	}{
		{map[string]any{}, shared.OnFailFatal, false},
		{map[string]any{"on_fail": nil}, shared.OnFailFatal, false},
		{map[string]any{"on_fail": ""}, shared.OnFailFatal, false},
		{map[string]any{"on_fail": "warn"}, shared.OnFailWarn, false},
		{map[string]any{"on_fail": "fatal"}, shared.OnFailFatal, false},
		{map[string]any{"on_fail": "stop"}, shared.OnFailStop, false},
		{map[string]any{"on_fail": "WARN"}, shared.OnFailFatal, true},
		{map[string]any{"on_fail": "continue"}, shared.OnFailFatal, true},
		{map[string]any{"on_fail": 1}, shared.OnFailFatal, true},
	}
	for _, tc := range cases {
		got, err := parseOnFail(tc.raw)
		assert.Equal(t, tc.want, got, "%v", tc.raw)
		if tc.wantErr {
			assert.Error(t, err, "%v", tc.raw)
		} else {
			assert.NoError(t, err, "%v", tc.raw)
		}
	}
}

func TestResult_Summary(t *testing.T) {
	f := &TaskFailure{Sequence: 3, Name: "Announce", Task: "slack_send", Msg: "boom"}
	cases := []struct {
		r    Result
		want string
	}{
		{Result{Outcome: OutcomeCompleted}, "All tasks complete. Exiting with code 0."},
		{Result{Outcome: OutcomeWarnings, Warnings: []TaskFailure{*f, *f}}, "Completed with 2 warning(s). Exiting with code 0."},
		{Result{Outcome: OutcomeStopped, StoppedAt: f}, "Workflow stopped at task 3 \"Announce\" [slack_send]: boom. Exiting with code 0."},
		{Result{Outcome: OutcomeFatal, Fatal: f}, "Terminating due to fatal error in task 3 \"Announce\" [slack_send]. Exiting with code 1."},
		{Result{Outcome: OutcomeFatal, Fatal: &TaskFailure{Sequence: 1, Task: "x"}}, "Terminating due to fatal error in task 1 [x]. Exiting with code 1."},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, tc.r.Summary())
	}
}

func TestExitIfTaskStops(t *testing.T) {
	rec := &recorder{}
	shared.SetVar("deploy_pending", "no")
	r := run(t, rec,
		map[string]any{
			"name":   "exit when nothing pending",
			"task":   "exit_if",
			"select": []map[string]any{{"field": "deploy_pending", "value": "no", "compare": "equal"}},
		},
		task("succeed", nil),
	)
	assert.Equal(t, OutcomeStopped, r.Outcome)
	assert.Equal(t, 0, r.ExitCode())
	assert.Nil(t, ran)
	require.NotNil(t, r.StoppedAt)
	assert.Equal(t, 1, r.StoppedAt.Sequence)
}

func TestAlert(t *testing.T) {
	w := New()
	w.Name = "Nightly"
	warn := TaskFailure{Sequence: 1, Name: "a", Task: "slack_send", Msg: "w"}
	fatal := TaskFailure{Sequence: 3, Name: "c", Task: "aws_asg_refresh", Msg: "f"}
	stop := TaskFailure{Sequence: 2, Name: "b", Task: "exit_if", Msg: "done"}

	a := w.Alert(Result{Outcome: OutcomeCompleted, Started: time.Unix(100, 0)})
	assert.Equal(t, notify.SeverityCompleted, a.Severity)
	assert.False(t, a.IsAlert())
	assert.Empty(t, a.Failures)
	assert.Equal(t, "Nightly", a.Workflow)
	assert.NotEmpty(t, a.Host)
	assert.Equal(t, time.Unix(100, 0), a.Started)

	a = w.Alert(Result{Outcome: OutcomeStopped, Warnings: []TaskFailure{warn}, StoppedAt: &stop})
	assert.Equal(t, notify.SeverityStopped, a.Severity)
	assert.False(t, a.IsAlert())
	assert.Equal(t, []notify.Failure{
		{Severity: notify.SeverityWarning, Sequence: 1, Name: "a", Task: "slack_send", Msg: "w"},
		{Severity: notify.SeverityStopped, Sequence: 2, Name: "b", Task: "exit_if", Msg: "done"},
	}, a.Failures)

	a = w.Alert(Result{Outcome: OutcomeWarnings, Warnings: []TaskFailure{warn}, TasksRun: 4, TasksTotal: 4})
	assert.Equal(t, notify.SeverityWarning, a.Severity)
	assert.True(t, a.IsAlert())
	assert.Equal(t, []notify.Failure{{Severity: notify.SeverityWarning, Sequence: 1, Name: "a", Task: "slack_send", Msg: "w"}}, a.Failures)
	assert.Equal(t, 0, a.TasksNotRun)

	a = w.Alert(Result{Outcome: OutcomeFatal, Warnings: []TaskFailure{warn}, Fatal: &fatal, TasksRun: 3, TasksTotal: 6})
	assert.Equal(t, notify.SeverityFatal, a.Severity)
	assert.True(t, a.IsAlert())
	require.Len(t, a.Failures, 2)
	assert.Equal(t, notify.SeverityWarning, a.Failures[0].Severity)
	assert.Equal(t, notify.SeverityFatal, a.Failures[1].Severity)
	assert.Equal(t, 3, a.Failures[1].Sequence)
	assert.Equal(t, 3, a.TasksNotRun)

	w.Name = ""
	a = w.Alert(Result{Outcome: OutcomeFatal, Fatal: &fatal})
	assert.Equal(t, defaultName, a.Workflow)
}

// The failure recorded for a notification carries the task's own message; the
// "(continuing ...)" note is added to the console copy only.
func TestExecute_RecordedWarningMessageHasNoContinuingNote(t *testing.T) {
	rec := &recorder{}
	r := run(t, rec, task("fail", "warn"), task("succeed", nil))
	assert.Equal(t, OutcomeWarnings, r.Outcome)
	require.Len(t, r.Warnings, 1)
	assert.Equal(t, "simulated failure", r.Warnings[0].Msg)
	assert.Equal(t, "simulated failure (continuing: on_fail is warn)", rec.results[0].Msg)
}

func TestLoad_NameAndNotify(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
env: /tmp/x.env
notify:
  slack:
    env_suffix: _ALERTS
    mention: here
  email:
    to: [a@example.com]
    from: b@example.com
    subject_prefix: "[X]"
    transcript: error
tasks:
  - task: test_fake
    on_fail: warn
`), 0o600))

	w := New()
	require.NoError(t, w.Load(path))
	assert.Equal(t, "deploy.yaml", w.Name, "name defaults to the file basename")
	require.NotNil(t, w.Notify.Slack)
	assert.Equal(t, "_ALERTS", w.Notify.Slack.EnvSuffix)
	assert.Equal(t, "here", w.Notify.Slack.Mention)
	require.NotNil(t, w.Notify.Email)
	assert.Equal(t, []string{"a@example.com"}, w.Notify.Email.To)
	assert.Equal(t, "[X]", w.Notify.Email.SubjectPrefix)
	assert.Equal(t, notify.TranscriptError, w.Notify.Email.Transcript)
	assert.True(t, w.Notify.Enabled())
	require.Len(t, w.Tasks, 1)
	assert.Equal(t, "warn", w.Tasks[0]["on_fail"])

	require.NoError(t, os.WriteFile(path, []byte("name: Named\ntasks: []\n"), 0o600))
	w = New()
	require.NoError(t, w.Load(path))
	assert.Equal(t, "Named", w.Name)
	assert.False(t, w.Notify.Enabled())
}

func TestLoad_TranscriptDefaultsAndValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.yaml")

	require.NoError(t, os.WriteFile(path, []byte("notify:\n  email:\n    to: [a@example.com]\n    from: b@example.com\ntasks: []\n"), 0o600))
	w := New()
	require.NoError(t, w.Load(path))
	assert.Equal(t, notify.TranscriptAlways, w.Notify.Email.Transcript, "transcript defaults to always")

	require.NoError(t, os.WriteFile(path, []byte("notify:\n  email:\n    to: [a@example.com]\n    from: b@example.com\n    transcript: sometimes\ntasks: []\n"), 0o600))
	w = New()
	err := w.Load(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid notify block")
	assert.Contains(t, err.Error(), `got "sometimes"`)
}
