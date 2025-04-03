// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"regexp"
)

// ResolveTags attempts, on a best effort basis, to turn tags in a string into
// jira markdown with user account IDs to tag the users.
func (j *CloudJira) ResolveTags(s string) string {
	re := regexp.MustCompile(`\[([^\]]+@[^\]]+)\]`)
	return re.ReplaceAllStringFunc(s, func(m string) string {
		parts := re.FindStringSubmatch(m)
		if len(parts) == 2 {
			return j.bestEffortReplace(parts[1])
		}
		return m
	})
}

func (j *CloudJira) bestEffortReplace(s string) string {
	accountID, err := j.GetUser(s)
	if err == nil {
		if accountID != "" {
			return "[~accountid:" + accountID + "]"
		}
	}
	// If we can't find the user, just return the original string with bold markup
	return "*" + s + "*"
}
