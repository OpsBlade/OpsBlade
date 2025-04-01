// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"github.com/andygrunwald/go-jira"
)

func (j *CloudJira) Client() (*jira.Client, error) {
	jt := jira.BasicAuthTransport{
		Username: j.Config.Username,
		Password: j.Config.Token,
	}

	client, err := jira.NewClient(jt.Client(), j.Config.BaseURL)
	if err != nil {
		return client, err
	}

	return client, nil
}
