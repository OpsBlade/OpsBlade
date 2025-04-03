// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package comment

import (
	"encoding/json"
	"fmt"

	"github.com/andygrunwald/go-jira"

	"github.com/OpsBlade/OpsBlade/services/cloudjira"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context     shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials `yaml:"credentials" json:"credentials"` // Allow override of credentials
	IssueId     string             `yaml:"issue_id" json:"issue_id"`       // Jira Issue ID
	Comment     string             `yaml:"comment" json:"comment"`         // Comment to add
}

func init() {
	shared.RegisterTask("jira_issue_comment", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	if t.IssueId == "" || t.Comment == "" {
		return t.Context.Error("issue_id and comment are required", nil)
	}

	creds := shared.NewCredentials(t.Credentials, *t.Context.Credentials)

	jiraClientConfig, err := cloudjira.New(
		cloudjira.WithUsername(creds.JIRA.Username),
		cloudjira.WithToken(creds.JIRA.Password),
		cloudjira.WithBaseURL(creds.JIRA.BaseURL))
	if err != nil {
		return t.Context.Error("failed to create JIRA client", err)
	}

	// Create a JIRA client
	client, err := jiraClientConfig.Client()
	if err != nil {
		return t.Context.Error("unable to create JIRA client", err)
	}

	// Create a jira comment object
	comment := &jira.Comment{
		Body: jiraClientConfig.ResolveTags(t.Comment),
	}

	// Add the comment to the issue
	_, _, err = client.Issue.AddComment(t.IssueId, comment)
	if err != nil {
		return t.Context.Error("failed to add comment to JIRA issue", err)
	}

	return t.Context.Result(true, fmt.Sprintf("comment added to JIRA issue %s", t.IssueId), nil)
}
