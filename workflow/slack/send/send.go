// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package send

import (
	"encoding/json"

	"github.com/OpsBlade/OpsBlade/services/cloudslack"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context     shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials `yaml:"credentials" json:"credentials"` // Allow override of credentials
	Subject     string             `yaml:"subject" json:"subject"`         // Subject of the message
	Body        string             `yaml:"body" json:"body"`               // Body of the message
}

func init() {
	shared.RegisterTask("slack_send", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error
	data := make(map[string]string)

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	// Debug information
	if t.Context.Debug {
		shared.DumpTask(t)
	}

	// Resolve credentials with priority to task credentials, then context credentials
	creds := shared.NewCredentials(t.Credentials, *t.Context.Credentials)

	// Create a new Slack client - note that it will try to read a webhook from the environment
	// and a ~/.slack file, but passing a webhook will override it
	s, err := cloudslack.New(cloudslack.WithWebhook(creds.Slack.Webhook), cloudslack.WithDebug(t.Context.Debug))
	if err != nil {
		return t.Context.Error("failed to create Slack client", err)
	}

	if t.Context.DryRun {
		return t.Context.Result(true, "DryRun, no message sent", data)
	}

	err = s.SendMessage(t.Subject, t.Body)
	if err != nil {
		return t.Context.Error("failed to send Slack message", err)
	}

	data["slack_subject"] = t.Subject
	data["slack_body"] = t.Body
	return t.Context.Result(true, "Slack message sent", data)
}
