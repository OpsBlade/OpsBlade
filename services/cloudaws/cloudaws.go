// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudaws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

type CloudAWS struct {
	Config *AWSConfig
	AWS    *aws.Config
}

type AWSConfig struct {
	Profile    string
	ConfigFile string
	CredsFile  string
	Region     string
	AccessKey  string
	SecretKey  string
}

type Option func(*AWSConfig)

func New(options ...Option) (*CloudAWS, error) {
	var cfg *AWSConfig

	// Check to see if the environment is already loaded
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" || os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		// Initialize AWSConfig with default paths for config and credentials files
		cfg = &AWSConfig{
			ConfigFile: filepath.Join(os.Getenv("HOME"), ".aws", "config"),
			CredsFile:  filepath.Join(os.Getenv("HOME"), ".aws", "credentials"),
		}
	} else {
		// Initialize AWSConfig with default values from environment variables
		cfg = &AWSConfig{
			AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
			SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			Region:    os.Getenv("AWS_REGION")}
	}

	// Apply user-provided options to override default values
	for _, opt := range options {
		opt(cfg)
	}

	var awsCfg aws.Config
	var err error

	// If access key and secret key are provided, use static credentials
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
			config.WithRegion(cfg.Region),
		)
	} else if cfg.Profile != "" { // If profile is provided, use shared config profile
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithSharedConfigProfile(cfg.Profile),
			config.WithSharedConfigFiles([]string{cfg.ConfigFile, cfg.CredsFile}),
			config.WithRegion(cfg.Region),
		)
	} else { // Otherwise, use the default credential provider chain
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(cfg.Region),
		)
	}

	// Return an error if loading the configuration fails
	if err != nil {
		return nil, err
	}

	// Return the CloudAWS struct with the loaded configuration
	return &CloudAWS{AWS: &awsCfg, Config: cfg}, nil
}

func WithProfile(profile string) Option {
	return func(cfg *AWSConfig) {
		if profile != "" {
			cfg.Profile = profile
		}
	}
}

func WithConfigFile(configFile string) Option {
	return func(cfg *AWSConfig) {
		if configFile != "" {
			cfg.ConfigFile = configFile
		}
	}
}

func WithCredsFile(credsFile string) Option {
	return func(cfg *AWSConfig) {
		if credsFile != "" {
			cfg.CredsFile = credsFile
		}
	}
}

func WithRegion(region string) Option {
	return func(cfg *AWSConfig) {
		if region != "" {
			cfg.Region = region
		}
	}
}

func WithAccessKey(accessKey string) Option {
	return func(cfg *AWSConfig) {
		if accessKey != "" {
			cfg.AccessKey = accessKey
		}
	}
}

func WithSecretKey(secretKey string) Option {
	return func(cfg *AWSConfig) {
		if secretKey != "" {
			cfg.SecretKey = secretKey
		}
	}
}

func (c *CloudAWS) Dump() {
	// Marshall the configuration to JSON and print it
	data, err := json.MarshalIndent(c.Config, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal CloudAWS config: %s\n", err.Error())
		return
	}
	fmt.Printf("cloudaws:\n%s\n\n", string(data))
}
