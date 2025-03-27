// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package createIssue

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
	Project     string             `yaml:"project" json:"project"`         // JIRA project key
	IssueType   string             `yaml:"issue_type" json:"issue_type"`   // JIRA issue type
	Summary     string             `yaml:"summary" json:"summary"`         // Summary of the issue
	Description string             `yaml:"description" json:"description"` // Description of the issue
	Assignee    string             `yaml:"assignee" json:"assignee"`       // Assignee email address or "" for default assignment
	Fields      []string           `yaml:"fields" json:"fields"`           // List of fields to return as data
}

func init() {
	shared.RegisterTask("jira_issue_create", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error
	var userAccountID = ""
	data := make(map[string]string)

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	creds := shared.NewCredentials(t.Credentials, *t.Context.Credentials)

	jiraClientConfig, err := cloudjira.New(
		cloudjira.WithUsername(creds.JIRA.Username),
		cloudjira.WithToken(creds.JIRA.Password),
		cloudjira.WithBaseURL(creds.JIRA.BaseURL))
	if err != nil {
		return t.Context.Error("failed to create JIRA client", err)
	}

	// Resolve assignee before creating issue in case of error
	if t.Assignee != "" {
		userAccountID, err = jiraClientConfig.GetUser(t.Assignee)
		if err != nil {
			return t.Context.Error(fmt.Sprintf("unable to resolve JIRA user '%s'", t.Assignee), err)
		}

		if t.Context.Debug {
			fmt.Printf("Assignee '%s' resolved to AccountID %s\n", t.Assignee, userAccountID)
		}
	}

	client, err := jiraClientConfig.Client()
	if err != nil {
		return t.Context.Error("unable to create JIRA client", err)
	}

	jiraIssue := jira.Issue{
		Fields: &jira.IssueFields{
			Description: t.Description,
			Type: jira.IssueType{
				Name: t.IssueType,
			},
			Project: jira.Project{
				Key: t.Project,
			},
			Summary: t.Summary,
		},
	}

	if t.Context.DryRun {
		shared.SetVar("jira_issue_id", "jira-issue-dry-run")
	} else {
		createdIssue, _, issueErr := client.Issue.Create(&jiraIssue)
		if issueErr != nil {
			return t.Context.Error("failed to create JIRA issue", issueErr)
		}

		if t.Context.Debug {
			fmt.Printf("Created JIRA issue '%s'\n", createdIssue.Key)
		}

		// Save the created issue ID
		shared.SetVar("jira_issue_id", createdIssue.Key)
		data["jira_issue_id"] = createdIssue.Key

		// If an assignee is provided, assign the issue
		if t.Assignee != "" {
			_, assignErr := client.Issue.UpdateAssignee(createdIssue.ID, &jira.User{
				AccountID: userAccountID,
			})

			if assignErr != nil {
				return t.Context.Error("failed to update JIRA user assignee", assignErr)
			}

			if t.Context.Debug {
				fmt.Printf("Assigned JIRA issue '%s' to user '%s' (%s)\n",
					createdIssue.Key, t.Assignee, userAccountID)
			}
			data["jira_assignee"] = t.Assignee
			data["jira_assignee_account_id"] = userAccountID
		}
	}
	return t.Context.Result(true, fmt.Sprintf("JIRA issue %s created", data), data)
}
