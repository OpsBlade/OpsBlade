// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
)

func (j *CloudJira) GetSprintCustomField() (string, error) {

	// Get the list of fields
	fields, err := j.GetFields()
	if err != nil {
		return "", err
	}

	// Iterate through the fields to find the sprint field
	for _, field := range fields {
		if field.Schema.Custom == "com.pyxis.greenhopper.jira:gh-sprint" {
			return field.Key, nil
		}
	}

	return "", fmt.Errorf("sprint field not found")
}

func (j *CloudJira) GetFields() ([]jira.Field, error) {
	client, err := j.Client()
	if err != nil {
		return []jira.Field{}, err
	}

	fields, _, err := client.Field.GetList()
	if err != nil {
		return []jira.Field{}, err
	}

	// Return the issue key
	return fields, nil
}
