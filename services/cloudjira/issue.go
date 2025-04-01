package cloudjira

import (
	"fmt"
	"github.com/andygrunwald/go-jira"
)

func (j *CloudJira) GetIssue(issueID string) (*jira.Issue, error) {
	client, err := j.Client()
	if err != nil {
		return nil, err
	}

	issue, _, err := client.Issue.Get(issueID, nil)
	if err != nil {
		return nil, err
	}

	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}

	// Return the issue key
	return issue, nil
}
