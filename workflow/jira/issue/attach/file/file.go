// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package file

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/OpsBlade/OpsBlade/services/cloudjira"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context     shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials `yaml:"credentials" json:"credentials"` // Allow override of credentials
	IssueId     string             `yaml:"issue_id" json:"issue_id"`       // Jira Issue ID
	FileName    string             `yaml:"file_name" json:"file_name"`     // File to add
}

func init() {
	shared.RegisterTask("jira_issue_attach_file", func(context shared.TaskContext) shared.Task {
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

	if t.IssueId == "" || t.FileName == "" {
		return t.Context.Error("issue_id and file_name are required", nil)
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

	// Open the file
	file, err := os.Open(t.FileName)
	if err != nil {
		return t.Context.Error("failed to open file", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	// Get the file name only
	fileNameOnly := t.FileName
	if lastSlash := strings.LastIndex(t.FileName, string(os.PathSeparator)); lastSlash != -1 {
		fileNameOnly = t.FileName[lastSlash+1:]
	}

	// Add the file to the issue
	_, _, err = client.Issue.PostAttachment(t.IssueId, file, fileNameOnly)
	if err != nil {
		return t.Context.Error("failed to attach file to JIRA issue", err)
	}

	return t.Context.Result(true, fmt.Sprintf("file attached to JIRA issue %s", t.IssueId),
		map[string]any{"jira_attached_file_name": fileNameOnly})
}
