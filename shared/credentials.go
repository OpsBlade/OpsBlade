// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

// Credentials holds all credentials and must be handled with care. YAML tags are needed to parse the input file,
// but JSON is suppressed to help avoid leaking sensitive information.

type Credentials struct {
	AWS     AWSCreds     `yaml:"aws" json:"-"`
	JIRA    JiraCreds    `yaml:"jira" json:"-"`
	Slack   SlackCreds   `yaml:"slack" json:"-"`
	Example ExampleCreds `yaml:"example" json:"-"`
}

type AWSCreds struct {
	Region     string `yaml:"region" json:"region" json:"-"`
	AccessKey  string `yaml:"access_key" json:"access_key" json:"-"`
	SecretKey  string `yaml:"secret_key" json:"secret_key" json:"-"`
	Profile    string `yaml:"profile" json:"profile"  json:"-"`
	ConfigFile string `yaml:"config_file" json:"config_file" json:"-"`
	CredsFile  string `yaml:"creds_file" json:"creds_file" json:"-"`
}

type JiraCreds struct {
	Username string `yaml:"username" json:"username" json:"-"`
	Password string `yaml:"password" json:"password" json:"-"`
	BaseURL  string `yaml:"base_url" json:"base_url" json:"-"`
}

type SlackCreds struct {
	Webhook string `yaml:"webhook" json:"webhook" json:"-"`
}

type ExampleCreds struct{}

func NewCredentials(taskCredentials Credentials, contextCredentials Credentials) Credentials {
	var creds = Credentials{}

	if OneField(taskCredentials.AWS) {
		creds.AWS = taskCredentials.AWS
	} else if OneField(contextCredentials.AWS) {
		creds.AWS = contextCredentials.AWS
	}

	if OneField(taskCredentials.JIRA) {
		creds.JIRA = taskCredentials.JIRA
	} else if OneField(contextCredentials.JIRA) {
		creds.JIRA = contextCredentials.JIRA
	}

	return creds
}
