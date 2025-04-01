// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
	"path/filepath"
)

type CloudJira struct {
	Config JiraConfig
}

type JiraConfig struct {
	Username string
	Token    string
	BaseURL  string
}

type Option func(*JiraConfig)

func New(options ...Option) (*CloudJira, error) {

	// Check to see if the environment is already loaded
	if os.Getenv("JIRA_USER") == "" || os.Getenv("JIRA_TOKEN") == "" || os.Getenv("JIRA_URL") == "" {
		// Load environment variables from the user's home/.jira file if it exists
		homeDir, err := os.UserHomeDir()
		if err == nil {
			jiraFilePath := filepath.Join(homeDir, ".jira")
			_ = godotenv.Load(jiraFilePath)
		}
	}

	// Initialize JiraConfig with default values from environment variables
	cfg := &JiraConfig{
		Username: os.Getenv("JIRA_USER"),
		Token:    os.Getenv("JIRA_TOKEN"),
		BaseURL:  os.Getenv("JIRA_URL"),
	}

	// Apply user-provided options to override default values
	for _, opt := range options {
		opt(cfg)
	}

	// Validate that required fields are set
	if cfg.Username == "" || cfg.Token == "" || cfg.BaseURL == "" {
		return nil, fmt.Errorf("missing required Jira configuration")
	}

	// Return the Jira struct with the loaded configuration
	return &CloudJira{
		Config: *cfg,
	}, nil
}

func WithUsername(username string) Option {
	return func(cfg *JiraConfig) {
		if username != "" {
			cfg.Username = username
		}
	}
}

func WithToken(token string) Option {
	return func(cfg *JiraConfig) {
		if token != "" {
			cfg.Token = token
		}
	}
}

func WithBaseURL(url string) Option {
	return func(cfg *JiraConfig) {
		if url != "" {
			cfg.BaseURL = url
		}
	}
}
