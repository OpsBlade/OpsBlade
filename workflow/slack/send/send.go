// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package send

import (
	"encoding/json"

	"github.com/OpsBlade/OpsBlade/services/cloudslack"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context   shared.TaskContext `yaml:"context" json:"context"`       // Task context
	Env       string             `yaml:"env" json:"env"`               // Optional file to load into the environment
	EnvSuffix string             `yaml:"env_suffix" json:"env_suffix"` // Optional suffix to append to SLACK_HOOK to allow more than one
	Subject   string             `yaml:"subject" json:"subject"`       // Subject of the message
	Body      string             `yaml:"body" json:"body"`             // Body of the message
	Pretty    []string           `yaml:"pretty" json:"pretty"`         // List of variables to append pretty-printed to the message
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

	// Prepare the message body
	msg := t.Body

	// Append pretty-printed variables to the message body
	var tmpAny any
	for _, v := range t.Pretty {
		tmpAny = shared.GetVar(v)
		if tmpAny != nil {
			msg += "\n```\n" + shared.AnyToYAMLIndent(tmpAny, "", 2) + "```"
		}
	}

	// Create a new Slack client - note that it will try to read a webhook from the environment
	// and a ~/.slack file, but passing a webhook will override it
	s, err := cloudslack.New(
		cloudslack.WithEnvironment(shared.SelectEnv(t.Env, t.Context.Env)),
		cloudslack.WithEnvSuffix(t.EnvSuffix),
		cloudslack.WithDebug(t.Context.Debug))
	if err != nil {
		return t.Context.Error("failed to create Slack client", err)
	}

	if t.Context.DryRun {
		return t.Context.Result(true, "DryRun, no message sent", data)
	}

	err = s.SendMessage(t.Subject, msg)
	if err != nil {
		return t.Context.Error("failed to send Slack message", err)
	}

	data["slack_subject"] = t.Subject
	data["slack_body"] = t.Body
	return t.Context.Result(true, "Slack message sent", data)
}
