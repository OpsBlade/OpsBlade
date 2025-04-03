// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import "fmt"

// GetUser retrieves a user ID by their email address from Jira.
func (j *CloudJira) GetUser(email string) (string, error) {
	client, err := j.Client()
	if err != nil {
		return "", err
	}

	users, _, err := client.User.Find(email)
	if err != nil {
		return "", err
	}

	if len(users) == 0 {
		return "", fmt.Errorf("user not found")
	}

	// Return the account ID of the first matching user
	return users[0].AccountID, nil
}
