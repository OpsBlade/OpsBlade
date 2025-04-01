// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"fmt"
	"io"

	"github.com/andygrunwald/go-jira"
)

func (j *CloudJira) ResponseToString(response *jira.Response) string {

	// Access the underlying *http.Response
	httpResponse := response.Response

	// Read the response body
	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return fmt.Sprintf("error reading response body: %s", err.Error())
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(httpResponse.Body)

	return string(body)
}
