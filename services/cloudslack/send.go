package cloudslack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type SlackMessage struct {
	Blocks []map[string]any `json:"blocks"`
}

func (s *CloudSlack) SendMessage(subject, message string) error {
	var err error

	payload := SlackMessage{
		Blocks: []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": "*" + subject + "*",
				},
			},
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": message,
				},
			},
		},
	}

	// Serialize the payload to JSON
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Set up the HTTP POST request
	req, err := http.NewRequest("POST", s.Config.Webhook, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// Send it
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("non-200 response from Slack: %s", resp.Status)
	}
	return nil
}
