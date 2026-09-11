// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

// Package notify sends workflow-level notifications to Slack and/or email. WARNING
// and FATAL ERROR alerts go to every configured channel; COMPLETED and STOPPED
// notifications carry the run transcript and go to email only, when asked for.
// Channels are attempted independently; a failure in one does not suppress the other.
package notify

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/OpsBlade/OpsBlade/services/cloudemail"
	"github.com/OpsBlade/OpsBlade/services/cloudslack"
	"github.com/OpsBlade/OpsBlade/shared"
)

// Config is the workflow-level notify block
type Config struct {
	Slack *SlackConfig `yaml:"slack" json:"slack"`
	Email *EmailConfig `yaml:"email" json:"email"`
}

// SlackConfig selects the alert webhook. The URL itself comes from the environment.
type SlackConfig struct {
	Env       string `yaml:"env" json:"env"`               // Optional env file, overrides the workflow env
	EnvSuffix string `yaml:"env_suffix" json:"env_suffix"` // Optional, reads SLACK_WEBHOOK<suffix>
	Mention   string `yaml:"mention" json:"mention"`       // channel (default), here, or none
}

// EmailConfig selects recipients. Server settings come from the environment.
type EmailConfig struct {
	Env           string   `yaml:"env" json:"env"`
	To            []string `yaml:"to" json:"to"`
	From          string   `yaml:"from" json:"from"`
	SubjectPrefix string   `yaml:"subject_prefix" json:"subject_prefix"` // Applied to every email
	Transcript    string   `yaml:"transcript" json:"transcript"`         // never, error, or always (default)
}

// Valid values for the email transcript field
const (
	TranscriptNever  = "never"  // alert emails only, without the transcript
	TranscriptError  = "error"  // alert emails carry the transcript; nothing is sent otherwise
	TranscriptAlways = "always" // every run sends an email carrying the transcript (default)
)

// Validate applies defaults and rejects values that would otherwise only be
// noticed when a notification is sent
func (c *Config) Validate() error {
	if c.Email == nil {
		return nil
	}
	switch c.Email.Transcript {
	case "":
		c.Email.Transcript = TranscriptAlways
	case TranscriptNever, TranscriptError, TranscriptAlways:
	default:
		return fmt.Errorf("notify.email.transcript must be %q, %q, or %q, got %q",
			TranscriptNever, TranscriptError, TranscriptAlways, c.Email.Transcript)
	}
	return nil
}

// Enabled reports whether any channel is configured
func (c Config) Enabled() bool {
	return c.Slack != nil || c.Email != nil
}

// transcriptMode returns the transcript setting with the default applied
func (c *EmailConfig) transcriptMode() string {
	if c.Transcript == "" {
		return TranscriptAlways
	}
	return c.Transcript
}

// Severity of a notification. WARNING and FATAL ERROR are alerts; COMPLETED and
// STOPPED describe uneventful runs and are only sent as transcript emails.
type Severity string

const (
	SeverityCompleted Severity = "COMPLETED"
	SeverityStopped   Severity = "STOPPED"
	SeverityWarning   Severity = "WARNING"
	SeverityFatal     Severity = "FATAL ERROR"
)

// Failure is one task entry in a notification: a warned task, the fatal task,
// or the task that stopped the workflow cleanly
type Failure struct {
	Severity Severity // WARNING, FATAL ERROR, or STOPPED
	Sequence int
	Name     string
	Task     string
	Msg      string
}

// Alert is the content of a notification
type Alert struct {
	Severity    Severity
	Workflow    string
	Host        string
	Started     time.Time
	Failures    []Failure
	TasksNotRun int    // FATAL only
	Transcript  string // Captured stdout and stderr of the run, if available
}

// IsAlert reports whether the severity warrants a loud alert
func (a Alert) IsAlert() bool {
	return a.Severity == SeverityWarning || a.Severity == SeverityFatal
}

// SlackSender sends a subject and mrkdwn body
type SlackSender interface {
	SendMessage(subject, message string) error
}

// EmailSender sends a plain-text message
type EmailSender interface {
	Send(from string, to []string, subject, body string) error
}

// Notifier dispatches alerts according to a Config
type Notifier struct {
	cfg       Config
	globalEnv string
	dryRun    bool
	debug     bool
	out       io.Writer
	newSlack  func(env, suffix string) (SlackSender, error)
	newEmail  func(env string) (EmailSender, error)
}

type Option func(*Notifier)

// New creates a Notifier. Senders are constructed lazily so that a misconfigured
// channel is reported as a send error rather than a construction error.
func New(cfg Config, options ...Option) *Notifier {
	n := &Notifier{
		cfg: cfg,
		out: os.Stdout,
		newSlack: func(env, suffix string) (SlackSender, error) {
			return cloudslack.New(cloudslack.WithEnvironment(env), cloudslack.WithEnvSuffix(suffix))
		},
		newEmail: func(env string) (EmailSender, error) {
			return cloudemail.New(cloudemail.WithEnvironment(env))
		},
	}
	for _, opt := range options {
		opt(n)
	}
	return n
}

// WithGlobalEnv sets the workflow-level env file used when a channel has none of its own
func WithGlobalEnv(env string) Option {
	return func(n *Notifier) { n.globalEnv = env }
}

// WithDryRun prints alerts instead of sending them
func WithDryRun(dryRun bool) Option {
	return func(n *Notifier) { n.dryRun = dryRun }
}

// WithDebug enables debug output
func WithDebug(debug bool) Option {
	return func(n *Notifier) { n.debug = debug }
}

// WithOutput sets where dry-run and debug output is written
func WithOutput(w io.Writer) Option {
	return func(n *Notifier) { n.out = w }
}

// WithSlackSender substitutes the Slack sender, for tests
func WithSlackSender(s SlackSender) Option {
	return func(n *Notifier) {
		n.newSlack = func(string, string) (SlackSender, error) { return s, nil }
	}
}

// WithEmailSender substitutes the email sender, for tests
func WithEmailSender(s EmailSender) Option {
	return func(n *Notifier) {
		n.newEmail = func(string) (EmailSender, error) { return s, nil }
	}
}

// Send delivers the notification to every channel that wants it and returns every
// error encountered. Slack receives alerts only. Email receives alerts, and also
// COMPLETED and STOPPED notifications when transcript is "always".
func (n *Notifier) Send(a Alert) []error {
	toSlack := n.cfg.Slack != nil && a.IsAlert()
	toEmail := n.cfg.Email != nil && (a.IsAlert() || n.cfg.Email.transcriptMode() == TranscriptAlways)
	if !toSlack && !toEmail {
		return nil
	}

	if n.dryRun {
		_, _ = fmt.Fprintf(n.out, "DryRun, notification not sent:\n%s", a.Text())
		if toEmail && n.includeTranscript(a) {
			_, _ = fmt.Fprintf(n.out, "\n(transcript omitted from dry run output)\n")
		}
		_, _ = fmt.Fprintln(n.out)
		return nil
	}

	var errs []error
	var slackErr error

	if toSlack {
		slackErr = n.sendSlack(a)
		if slackErr != nil {
			errs = append(errs, fmt.Errorf("slack alert failed: %w", slackErr))
		}
	}

	if toEmail {
		if err := n.sendEmail(a, slackErr); err != nil {
			errs = append(errs, fmt.Errorf("email notification failed: %w", err))
		}
	}
	return errs
}

func (n *Notifier) sendSlack(a Alert) error {
	cfg := n.cfg.Slack
	sender, err := n.newSlack(shared.SelectEnv(cfg.Env, n.globalEnv), cfg.EnvSuffix)
	if err != nil {
		return err
	}
	return sender.SendMessage(a.SlackSubject(cfg.Mention), a.SlackBody())
}

func (n *Notifier) sendEmail(a Alert, slackErr error) error {
	cfg := n.cfg.Email
	sender, err := n.newEmail(shared.SelectEnv(cfg.Env, n.globalEnv))
	if err != nil {
		return err
	}
	body := a.Text()
	if slackErr != nil {
		body += "\nSlack alert failed: " + slackErr.Error() + "\n"
	}
	if n.includeTranscript(a) {
		body += a.transcriptSection()
	}
	return sender.Send(cfg.From, cfg.To, a.EmailSubject(cfg.SubjectPrefix), body)
}

// includeTranscript reports whether the email for this notification carries the transcript
func (n *Notifier) includeTranscript(a Alert) bool {
	if n.cfg.Email == nil || a.Transcript == "" {
		return false
	}
	switch n.cfg.Email.transcriptMode() {
	case TranscriptNever:
		return false
	case TranscriptError:
		return a.IsAlert()
	default:
		return true
	}
}

// mentionPrefix returns the Slack mention for the configured setting
func mentionPrefix(mention string) string {
	switch strings.ToLower(strings.TrimSpace(mention)) {
	case "", "channel":
		return "<!channel> "
	case "here":
		return "<!here> "
	default:
		return ""
	}
}

func (a Alert) icon() string {
	if a.Severity == SeverityFatal {
		return ":rotating_light:"
	}
	return ":warning:"
}

// SlackSubject is the bold first line of the Slack message
func (a Alert) SlackSubject(mention string) string {
	return fmt.Sprintf("%s%s %s: %s", mentionPrefix(mention), a.icon(), a.Severity, a.Workflow)
}

// SlackBody is the mrkdwn body of the Slack message
func (a Alert) SlackBody() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Host: %s\nStarted: %s\n", a.Host, a.Started.Format(time.RFC3339)))
	for _, f := range a.Failures {
		b.WriteString(fmt.Sprintf("\n*%s:*\n```\n%s```\n", f.Severity, f.entry("")))
	}
	if a.Severity == SeverityFatal && a.TasksNotRun > 0 {
		b.WriteString(fmt.Sprintf("\n*CAUTION:* %d task(s) did not run.\n", a.TasksNotRun))
	}
	return b.String()
}

// EmailSubject is the email subject line
func (a Alert) EmailSubject(prefix string) string {
	if prefix != "" {
		return fmt.Sprintf("%s %s: %s", prefix, a.Severity, a.Workflow)
	}
	return fmt.Sprintf("%s: %s", a.Severity, a.Workflow)
}

// Text is the plain-text notification body used for email and dry-run output.
// Each task entry is a severity label followed by aligned lines: the task number
// and type, its name, what it reported, and what the workflow did about it.
func (a Alert) Text() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s: %s\nHost: %s\nStarted: %s\n", a.Severity, a.Workflow, a.Host, a.Started.Format(time.RFC3339)))
	for _, f := range a.Failures {
		b.WriteString(fmt.Sprintf("\n%s:\n%s", f.Severity, f.entry("    ")))
	}
	if a.Severity == SeverityFatal && a.TasksNotRun > 0 {
		b.WriteString(fmt.Sprintf("\nCAUTION: %d task(s) did not run.\n", a.TasksNotRun))
	}
	return b.String()
}

// transcriptSection is the separator block and transcript appended to an email body
func (a Alert) transcriptSection() string {
	rule := strings.Repeat("-", 70)
	transcript := a.Transcript
	if !strings.HasSuffix(transcript, "\n") {
		transcript += "\n"
	}
	return fmt.Sprintf("\n%s\nTranscript\n%s\n%s", rule, rule, transcript)
}

// String identifies the task
func (f Failure) String() string {
	if f.Name == "" {
		return fmt.Sprintf("Task %d [%s]", f.Sequence, f.Task)
	}
	return fmt.Sprintf("Task %d \"%s\" [%s]", f.Sequence, f.Name, f.Task)
}

// entry renders the task entry, each line prefixed by indent: the task number and
// type, the name when there is one, what it reported, and the action taken.
// Continuation lines of a multi-line message are aligned under the value column.
func (f Failure) entry(indent string) string {
	label, action := "Error: ", "continued"
	switch f.Severity {
	case SeverityFatal:
		action = "aborted"
	case SeverityStopped:
		label, action = "Reason:", "stopped"
	}
	lines := strings.Split(strings.TrimRight(f.Msg, "\n"), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" {
			lines[i] = indent + "        " + lines[i]
		}
	}
	msg := strings.Join(lines, "\n")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("%sTask:   %d - %s\n", indent, f.Sequence, f.Task))
	if f.Name != "" {
		b.WriteString(fmt.Sprintf("%sName:   \"%s\"\n", indent, f.Name))
	}
	b.WriteString(fmt.Sprintf("%s%s %s\n", indent, label, msg))
	b.WriteString(fmt.Sprintf("%sAction: %s\n", indent, action))
	return b.String()
}
