// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudslack"
)

var started = time.Date(2026, 9, 9, 22, 0, 0, 0, time.UTC)

func fatalAlert() Alert {
	return Alert{
		Severity: SeverityFatal,
		Workflow: "Nightly deploy",
		Host:     "ops1",
		Started:  started,
		Failures: []Failure{
			{Severity: SeverityWarning, Sequence: 2, Name: "Announce", Task: "slack_send", Msg: "non-200 response from Slack: 404 Not Found (channel_not_found)"},
			{Severity: SeverityFatal, Sequence: 5, Name: "Refresh", Task: "aws_asg_refresh", Msg: "boom\n\nSee runbook.\n"},
		},
		TasksNotRun: 3,
		Transcript:  "OpsBlade 0.2.00\n* Starting task 1\nTerminating due to fatal error. Exiting with code 1.\n",
	}
}

func completedAlert() Alert {
	return Alert{Severity: SeverityCompleted, Workflow: "Nightly deploy", Host: "ops1", Started: started, Transcript: "all good\n"}
}

func stoppedAlert() Alert {
	return Alert{
		Severity:   SeverityStopped,
		Workflow:   "Nightly deploy",
		Host:       "ops1",
		Started:    started,
		Failures:   []Failure{{Severity: SeverityStopped, Sequence: 3, Name: "Check ticket", Task: "jira_issue_check", Msg: "JIRA issue OPS-1 is in status 'Open' but status 'Done' is required"}},
		Transcript: "stopped early\n",
	}
}

func emailConfig(transcript string) *EmailConfig {
	return &EmailConfig{To: []string{"a@example.com"}, From: "b@example.com", Transcript: transcript}
}

func warningAlert() Alert {
	return Alert{
		Severity:   SeverityWarning,
		Workflow:   "Nightly deploy",
		Host:       "ops1",
		Started:    started,
		Failures:   []Failure{{Severity: SeverityWarning, Sequence: 2, Name: "Announce", Task: "slack_send", Msg: "channel gone"}},
		Transcript: "one warning\n",
	}
}

// slackServer captures the webhook payload and answers with the given status and body
func slackServer(t *testing.T, status int, body string) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var blocks []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var payload struct {
			Blocks []map[string]any `json:"blocks"`
		}
		require.NoError(t, json.Unmarshal(raw, &payload))
		blocks = payload.Blocks
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &blocks
}

func blockText(b map[string]any) string {
	return b["text"].(map[string]any)["text"].(string)
}

type fakeEmail struct {
	from, subject, body string
	to                  []string
	err                 error
	calls               int
}

func (f *fakeEmail) Send(from string, to []string, subject, body string) error {
	f.calls++
	f.from, f.to, f.subject, f.body = from, to, subject, body
	return f.err
}

func TestSend_SlackFatalPayload(t *testing.T) {
	srv, blocks := slackServer(t, 200, "ok")
	t.Setenv("SLACK_WEBHOOK_ALERTS", srv.URL)
	slack, err := cloudslack.New(cloudslack.WithEnvSuffix("_ALERTS"))
	require.NoError(t, err)

	n := New(Config{Slack: &SlackConfig{EnvSuffix: "_ALERTS"}}, WithSlackSender(slack))
	errs := n.Send(fatalAlert())
	require.Empty(t, errs)

	require.Len(t, *blocks, 2)
	subject := blockText((*blocks)[0])
	body := blockText((*blocks)[1])
	assert.Equal(t, "*<!channel> :rotating_light: FATAL ERROR: Nightly deploy*", subject)
	assert.Contains(t, body, "Host: ops1")
	assert.Contains(t, body, "Started: 2026-09-09T22:00:00Z")
	assert.Contains(t, body, "*WARNING:*\n```\nTask:   2 - slack_send\nName:   \"Announce\"\nError:  non-200 response from Slack: 404 Not Found (channel_not_found)\nAction: continued\n```")
	assert.Contains(t, body, "*FATAL ERROR:*\n```\nTask:   5 - aws_asg_refresh\nName:   \"Refresh\"\nError:  boom\n\n        See runbook.\nAction: aborted\n```")
	assert.Contains(t, body, "*CAUTION:* 3 task(s) did not run.")
	assert.NotContains(t, body, "Transcript", "slack never carries the transcript")
}

func TestSend_SlackWarningPayload(t *testing.T) {
	srv, blocks := slackServer(t, 200, "ok")
	t.Setenv("SLACK_WEBHOOK", srv.URL)
	slack, err := cloudslack.New()
	require.NoError(t, err)

	n := New(Config{Slack: &SlackConfig{Mention: "here"}}, WithSlackSender(slack))
	require.Empty(t, n.Send(warningAlert()))

	subject := blockText((*blocks)[0])
	body := blockText((*blocks)[1])
	assert.Equal(t, "*<!here> :warning: WARNING: Nightly deploy*", subject)
	assert.NotContains(t, body, "did not run")
}

func TestSend_SlackChannelNotFound(t *testing.T) {
	srv, _ := slackServer(t, 404, "channel_not_found")
	t.Setenv("SLACK_WEBHOOK", srv.URL)
	slack, err := cloudslack.New()
	require.NoError(t, err)

	n := New(Config{Slack: &SlackConfig{}}, WithSlackSender(slack))
	errs := n.Send(warningAlert())
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "slack alert failed")
	assert.Contains(t, errs[0].Error(), "channel_not_found")
}

func TestSend_SlackNotConfiguredInEnv(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK", "")
	n := New(Config{Slack: &SlackConfig{}})
	errs := n.Send(warningAlert())
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "webhook is not configured")
}

func TestSend_EmailOnly(t *testing.T) {
	fe := &fakeEmail{}
	cfg := Config{Email: &EmailConfig{To: []string{"ops@example.com", "oncall@example.com"}, From: "opsblade@example.com", SubjectPrefix: "[OpsBlade]"}}
	n := New(cfg, WithEmailSender(fe))

	require.Empty(t, n.Send(fatalAlert()))
	assert.Equal(t, 1, fe.calls)
	assert.Equal(t, "opsblade@example.com", fe.from)
	assert.Equal(t, []string{"ops@example.com", "oncall@example.com"}, fe.to)
	assert.Equal(t, "[OpsBlade] FATAL ERROR: Nightly deploy", fe.subject)
	assert.Contains(t, fe.body, "FATAL ERROR: Nightly deploy")
	assert.Contains(t, fe.body, "Host: ops1")
	assert.Contains(t, fe.body, "\nFATAL ERROR:\n    Task:   5 - aws_asg_refresh\n    Name:   \"Refresh\"\n")
	assert.Contains(t, fe.body, "CAUTION: 3 task(s) did not run.")
	assert.NotContains(t, fe.body, "Slack alert failed")
	assert.Contains(t, fe.body, "Transcript", "transcript defaults to always")
}

func TestSend_EmailSubjectWithoutPrefix(t *testing.T) {
	fe := &fakeEmail{}
	n := New(Config{Email: &EmailConfig{To: []string{"a@example.com"}, From: "b@example.com"}}, WithEmailSender(fe))
	require.Empty(t, n.Send(warningAlert()))
	assert.Equal(t, "WARNING: Nightly deploy", fe.subject)
}

func TestSend_BothChannelsIndependent(t *testing.T) {
	// Slack fails; email must still be sent and must mention the Slack failure
	srv, _ := slackServer(t, 404, "channel_not_found")
	t.Setenv("SLACK_WEBHOOK", srv.URL)
	slack, err := cloudslack.New()
	require.NoError(t, err)
	fe := &fakeEmail{}

	cfg := Config{Slack: &SlackConfig{}, Email: &EmailConfig{To: []string{"a@example.com"}, From: "b@example.com"}}
	n := New(cfg, WithSlackSender(slack), WithEmailSender(fe))

	errs := n.Send(warningAlert())
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "slack alert failed")
	assert.Equal(t, 1, fe.calls)
	assert.Contains(t, fe.body, "Slack alert failed: ")
	assert.Contains(t, fe.body, "channel_not_found")
}

func TestSend_BothChannelsBothSucceed(t *testing.T) {
	srv, blocks := slackServer(t, 200, "ok")
	t.Setenv("SLACK_WEBHOOK", srv.URL)
	slack, err := cloudslack.New()
	require.NoError(t, err)
	fe := &fakeEmail{}

	cfg := Config{Slack: &SlackConfig{}, Email: &EmailConfig{To: []string{"a@example.com"}, From: "b@example.com"}}
	n := New(cfg, WithSlackSender(slack), WithEmailSender(fe))

	require.Empty(t, n.Send(warningAlert()))
	assert.Len(t, *blocks, 2)
	assert.Equal(t, 1, fe.calls)
	assert.NotContains(t, fe.body, "Slack alert failed")
}

func TestSend_BothFail(t *testing.T) {
	srv, _ := slackServer(t, 500, "")
	t.Setenv("SLACK_WEBHOOK", srv.URL)
	slack, err := cloudslack.New()
	require.NoError(t, err)
	fe := &fakeEmail{err: errors.New("relay refused")}

	cfg := Config{Slack: &SlackConfig{}, Email: &EmailConfig{To: []string{"a@example.com"}, From: "b@example.com"}}
	n := New(cfg, WithSlackSender(slack), WithEmailSender(fe))

	errs := n.Send(fatalAlert())
	require.Len(t, errs, 2)
	assert.Contains(t, errs[0].Error(), "slack alert failed")
	assert.Contains(t, errs[1].Error(), "email notification failed: relay refused")
}

func TestSend_DryRunPrintsAndSendsNothing(t *testing.T) {
	fe := &fakeEmail{}
	var out bytes.Buffer
	cfg := Config{Slack: &SlackConfig{}, Email: &EmailConfig{To: []string{"a@example.com"}, From: "b@example.com"}}
	n := New(cfg, WithDryRun(true), WithOutput(&out), WithEmailSender(fe))

	require.Empty(t, n.Send(fatalAlert()))
	assert.Equal(t, 0, fe.calls)
	assert.Contains(t, out.String(), "DryRun, notification not sent")
	assert.Contains(t, out.String(), "FATAL ERROR: Nightly deploy")
	assert.Contains(t, out.String(), "Task:   5 - aws_asg_refresh")
	assert.Contains(t, out.String(), "(transcript omitted from dry run output)")
	assert.NotContains(t, out.String(), "* Starting task 1", "dry run must not echo the transcript")
}

func TestSend_NothingConfigured(t *testing.T) {
	fe := &fakeEmail{}
	n := New(Config{}, WithEmailSender(fe))
	assert.Nil(t, n.Send(fatalAlert()))
	assert.Equal(t, 0, fe.calls)
	assert.False(t, Config{}.Enabled())
}

func TestMentionPrefix(t *testing.T) {
	assert.Equal(t, "<!channel> ", mentionPrefix(""))
	assert.Equal(t, "<!channel> ", mentionPrefix("channel"))
	assert.Equal(t, "<!channel> ", mentionPrefix(" Channel "))
	assert.Equal(t, "<!here> ", mentionPrefix("here"))
	assert.Equal(t, "", mentionPrefix("none"))
	assert.Equal(t, "", mentionPrefix("anything else"))
}

func TestText_Layout(t *testing.T) {
	want := `FATAL ERROR: Nightly deploy
Host: ops1
Started: 2026-09-09T22:00:00Z

WARNING:
    Task:   2 - slack_send
    Name:   "Announce"
    Error:  non-200 response from Slack: 404 Not Found (channel_not_found)
    Action: continued

FATAL ERROR:
    Task:   5 - aws_asg_refresh
    Name:   "Refresh"
    Error:  boom

            See runbook.
    Action: aborted

CAUTION: 3 task(s) did not run.
`
	assert.Equal(t, want, fatalAlert().Text())

	a := fatalAlert()
	a.TasksNotRun = 0
	assert.NotContains(t, a.Text(), "CAUTION", "no caution line when every task ran")

	assert.Equal(t, "COMPLETED: Nightly deploy\nHost: ops1\nStarted: 2026-09-09T22:00:00Z\n", completedAlert().Text())

	wantStopped := `STOPPED: Nightly deploy
Host: ops1
Started: 2026-09-09T22:00:00Z

STOPPED:
    Task:   3 - jira_issue_check
    Name:   "Check ticket"
    Reason: JIRA issue OPS-1 is in status 'Open' but status 'Done' is required
    Action: stopped
`
	assert.Equal(t, wantStopped, stoppedAlert().Text())

	// A task without a name has no Name line
	a = stoppedAlert()
	a.Failures[0].Name = ""
	assert.Contains(t, a.Text(), "STOPPED:\n    Task:   3 - jira_issue_check\n    Reason: ")
	assert.NotContains(t, a.Text(), "Name:")
}

func TestSend_EmailTranscriptSection(t *testing.T) {
	fe := &fakeEmail{}
	n := New(Config{Email: emailConfig(TranscriptAlways)}, WithEmailSender(fe))
	require.Empty(t, n.Send(fatalAlert()))
	rule := "----------------------------------------------------------------------"
	assert.Contains(t, fe.body, "CAUTION: 3 task(s) did not run.\n\n"+rule+"\nTranscript\n"+rule+"\nOpsBlade 0.2.00\n")
	assert.True(t, strings.HasSuffix(fe.body, "Exiting with code 1.\n"))

	// The Slack failure note stays above the transcript
	srv, _ := slackServer(t, 404, "channel_not_found")
	t.Setenv("SLACK_WEBHOOK", srv.URL)
	slack, err := cloudslack.New()
	require.NoError(t, err)
	fe = &fakeEmail{}
	n = New(Config{Slack: &SlackConfig{}, Email: emailConfig(TranscriptAlways)}, WithSlackSender(slack), WithEmailSender(fe))
	n.Send(warningAlert())
	assert.Less(t, strings.Index(fe.body, "Slack alert failed"), strings.Index(fe.body, "Transcript"))
}

func TestSend_TranscriptModes(t *testing.T) {
	cases := []struct {
		mode           string
		alert          Alert
		wantSent       bool
		wantTranscript bool
	}{
		{TranscriptNever, fatalAlert(), true, false},
		{TranscriptNever, warningAlert(), true, false},
		{TranscriptNever, completedAlert(), false, false},
		{TranscriptNever, stoppedAlert(), false, false},
		{TranscriptError, fatalAlert(), true, true},
		{TranscriptError, warningAlert(), true, true},
		{TranscriptError, completedAlert(), false, false},
		{TranscriptError, stoppedAlert(), false, false},
		{TranscriptAlways, fatalAlert(), true, true},
		{TranscriptAlways, warningAlert(), true, true},
		{TranscriptAlways, completedAlert(), true, true},
		{TranscriptAlways, stoppedAlert(), true, true},
		{"", completedAlert(), true, true}, // unset behaves as always
	}
	for _, tc := range cases {
		t.Run(tc.mode+"/"+string(tc.alert.Severity), func(t *testing.T) {
			fe := &fakeEmail{}
			cfg := Config{Email: emailConfig(tc.mode)}
			cfg.Email.SubjectPrefix = "[X]"
			n := New(cfg, WithEmailSender(fe))
			require.Empty(t, n.Send(tc.alert))
			if !tc.wantSent {
				assert.Equal(t, 0, fe.calls)
				return
			}
			require.Equal(t, 1, fe.calls)
			assert.Equal(t, "[X] "+string(tc.alert.Severity)+": Nightly deploy", fe.subject, "prefix applies to every email")
			assert.Equal(t, tc.wantTranscript, strings.Contains(fe.body, "\nTranscript\n"))
		})
	}
}

func TestSend_NoTranscriptAvailable(t *testing.T) {
	// A library caller that did not capture output gets the body without a separator block
	fe := &fakeEmail{}
	n := New(Config{Email: emailConfig(TranscriptAlways)}, WithEmailSender(fe))
	a := fatalAlert()
	a.Transcript = ""
	require.Empty(t, n.Send(a))
	assert.NotContains(t, fe.body, "Transcript")
}

func TestSend_SlackIgnoresNonAlerts(t *testing.T) {
	srv, blocks := slackServer(t, 200, "ok")
	t.Setenv("SLACK_WEBHOOK", srv.URL)
	slack, err := cloudslack.New()
	require.NoError(t, err)
	n := New(Config{Slack: &SlackConfig{}}, WithSlackSender(slack))
	require.Empty(t, n.Send(completedAlert()))
	require.Empty(t, n.Send(stoppedAlert()))
	assert.Empty(t, *blocks)
}

func TestConfigValidate(t *testing.T) {
	var c Config
	require.NoError(t, c.Validate(), "no email block is valid")

	c = Config{Email: emailConfig("")}
	require.NoError(t, c.Validate())
	assert.Equal(t, TranscriptAlways, c.Email.Transcript, "empty defaults to always")

	for _, v := range []string{TranscriptNever, TranscriptError, TranscriptAlways} {
		c = Config{Email: emailConfig(v)}
		require.NoError(t, c.Validate())
		assert.Equal(t, v, c.Email.Transcript)
	}

	c = Config{Email: emailConfig("Always")}
	err := c.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), `notify.email.transcript must be "never", "error", or "always", got "Always"`)
}

func TestFailureString(t *testing.T) {
	assert.Equal(t, `Task 2 "Announce" [slack_send]`, Failure{Sequence: 2, Name: "Announce", Task: "slack_send"}.String())
	assert.Equal(t, `Task 2 [slack_send]`, Failure{Sequence: 2, Task: "slack_send"}.String())
}

func TestSend_EnvSelection(t *testing.T) {
	var gotEnv, gotSuffix, gotEmailEnv string
	n := New(Config{Slack: &SlackConfig{EnvSuffix: "_X"}, Email: &EmailConfig{Env: "/email.env", To: []string{"a@example.com"}, From: "b@example.com"}},
		WithGlobalEnv("/global.env"))
	n.newSlack = func(env, suffix string) (SlackSender, error) {
		gotEnv, gotSuffix = env, suffix
		return nil, errors.New("stop here")
	}
	n.newEmail = func(env string) (EmailSender, error) {
		gotEmailEnv = env
		return nil, errors.New("stop here")
	}
	errs := n.Send(warningAlert())
	require.Len(t, errs, 2)
	assert.Equal(t, "/global.env", gotEnv, "slack falls back to the workflow env")
	assert.Equal(t, "_X", gotSuffix)
	assert.Equal(t, "/email.env", gotEmailEnv, "email uses its own env when set")
}
