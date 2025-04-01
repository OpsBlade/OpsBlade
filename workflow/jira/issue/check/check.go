// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package check

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OpsBlade/OpsBlade/services/cloudjira"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context            shared.TaskContext `yaml:"context" json:"context"`                         // Task context
	Credentials        shared.Credentials `yaml:"credentials" json:"credentials"`                 // Allow override of credentials
	IssueId            string             `yaml:"issue_id" json:"issue_id"`                       // Jira Issue ID
	RequiredStatus     string             `yaml:"required_status" json:"required_status"`         // State required to pass
	RequiredResolution string             `yaml:"required_resolution" json:"required_resolution"` // Resolution required to pass
}

func init() {
	shared.RegisterTask("jira_issue_check", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error
	data := make(map[string]any)

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	creds := shared.NewCredentials(t.Credentials, *t.Context.Credentials)

	data["check_jira_issue_id"] = t.IssueId
	data["check_jira_issue_required_status"] = t.RequiredStatus
	data["check_jira_issue_required_resolution"] = t.RequiredResolution

	jiraClientConfig, err := cloudjira.New(
		cloudjira.WithUsername(creds.JIRA.Username),
		cloudjira.WithToken(creds.JIRA.Password),
		cloudjira.WithBaseURL(creds.JIRA.BaseURL))
	if err != nil {
		return t.Context.Error("failed to create JIRA client", err)
	}

	// Get the issue
	issue, err := jiraClientConfig.GetIssue(t.IssueId)
	if err != nil {
		return t.Context.Error("failed to get JIRA issue", err)
	}

	if issue == nil {
		return t.Context.Error("failed to get JIRA issue", fmt.Errorf("issue is nil"))
	}

	// Obtain the status and resolution, but check for nil pointers
	var issueStatus string
	var issueResolution string
	if issue.Fields != nil {
		if issue.Fields.Status != nil {
			issueStatus = issue.Fields.Status.Name
		}
		if issue.Fields.Resolution != nil {
			issueResolution = issue.Fields.Resolution.Name
		}
	} else {
		return t.Context.Error("failed to get JIRA issue fields", err)
	}

	data["check_jira_issue_status"] = issueStatus
	data["check_jira_issue_resolution"] = issueResolution
	data["check_jira_issue_passed"] = false

	// Check if the issue is in the desired state
	if t.RequiredStatus != "" {
		if strings.ToLower(issue.Fields.Status.Name) != strings.ToLower(t.RequiredStatus) {
			return t.Context.Error(
				fmt.Sprintf("JIRA issue %s is in status '%s' not required status '%s'", t.IssueId, issue.Fields.Status.Name, t.RequiredStatus),
				nil)
		}
	}

	if t.RequiredResolution != "" {
		if strings.ToLower(issue.Fields.Resolution.Name) != strings.ToLower(t.RequiredResolution) {
			return t.Context.Error(
				fmt.Sprintf("JIRA issue %s is in resolution '%s' not required resolution '%s'", t.IssueId, issue.Fields.Resolution.Name, t.RequiredResolution),
				nil)
		}
	}

	data["check_jira_issue_passed"] = true
	return t.Context.Result(true, fmt.Sprintf("JIRA issue %s is in the desired state", t.IssueId), data)
}
