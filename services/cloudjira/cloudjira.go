// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type CloudJira struct {
	Config JiraConfig
}

type JiraConfig struct {
	Environment string
	Username    string
	Token       string
	BaseURL     string
}

type Option func(*JiraConfig)

func New(options ...Option) (*CloudJira, error) {
	cfg := &JiraConfig{}

	// Apply user-provided options to override default values
	for _, opt := range options {
		opt(cfg)
	}

	// If an environment file is provided, load it
	if cfg.Environment != "" {
		err := godotenv.Load(cfg.Environment)
		if err != nil {
			return nil, err
		}
	}

	// Load from environment variables
	cfg.Username = os.Getenv("JIRA_USER")
	cfg.Token = os.Getenv("JIRA_TOKEN")
	cfg.BaseURL = os.Getenv("JIRA_URL")

	// Validate that required fields are set
	if cfg.Username == "" || cfg.Token == "" || cfg.BaseURL == "" {
		return nil, fmt.Errorf("missing required Jira configuration")
	}

	// Return the Jira struct with the loaded configuration
	return &CloudJira{
		Config: *cfg,
	}, nil
}

func WithEnvironment(env string) Option {
	return func(cfg *JiraConfig) {
		if env != "" {
			cfg.Environment = env
		}
	}
}
