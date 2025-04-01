// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"fmt"

	"github.com/andygrunwald/go-jira"
)

// GetActiveSprint retrieves the active sprint for a given project in JIRA. If there is more than one,
// it returns the first one found.
func (j *CloudJira) GetActiveSprint(project string) (jira.Sprint, error) {
	var ret jira.Sprint

	// Obtain a JIRA client
	client, err := j.Client()
	if err != nil {
		return ret, err
	}

	// Find the board ID for the project
	boards, _, err := client.Board.GetAllBoards(&jira.BoardListOptions{
		ProjectKeyOrID: project,
	})
	if err != nil {
		return ret, err
	}

	// Check if any boards were found
	if len(boards.Values) == 0 {
		return ret, fmt.Errorf("no boards found for project %s", project)
	}

	// Use the first board found for the project
	boardID := boards.Values[0].ID

	// Get active sprints for the board
	sprintsList, _, err := client.Board.GetAllSprintsWithOptions(
		boardID,
		&jira.GetAllSprintsOptions{State: "active"})
	if err != nil {
		return ret, err
	}

	// Check if any active sprints were found
	if len(sprintsList.Values) == 0 {
		return ret, fmt.Errorf("no sprints found for board %d", boardID)
	}

	// Return the first active sprint found
	return sprintsList.Values[0], nil
}
