// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package wait

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"time"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context    shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Env        string             `yaml:"env" json:"env"`                 // Optional file to load into the environment
	Region     string             `yaml:"region" json:"region"`           // AWS region - allow overriding
	Profile    string             `yaml:"profile" json:"profile"`         // AWS profile - allow overriding
	ConfigFile string             `yaml:"config_file" json:"config_file"` // AWS config file - allow overriding
	ImageId    string             `yaml:"image_id" json:"image_id"`       // Instance ID to create AMI from
	Limit      int                `yaml:"limit" json:"limit"`             // Number of seconds to wait
}

func init() {
	shared.RegisterTask("aws_ec2_ami_wait", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data to Task", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	if t.ImageId == "" {
		return t.Context.Error("image_id is null, nothing to wait for", nil)
	}

	// Resolve credentials with priority to task credentials, then context credentials
	// Resolve environment file
	envFile := shared.SelectEnv(t.Env, t.Context.Env)

	amazonInstance, err := cloudaws.New(
		cloudaws.WithRegion(t.Region),
		cloudaws.WithEnvironment(envFile),
		cloudaws.WithProfile(t.Profile),
		cloudaws.WithConfigFile(t.ConfigFile))
	if err != nil || amazonInstance == nil {
		return t.Context.Error("failed to create AWS client", err)
	}

	client := amazonInstance.EC2Client()

	if t.Context.DryRun {
		return t.Context.Result(true, "DryRun, no image to wait for", nil)
	}

	// Wait for AMI to be available
	startTime := time.Now()

	for {
		if time.Since(startTime) > time.Duration(t.Limit)*time.Second {
			return t.Context.Error(
				fmt.Sprintf("timeout, image %s not available after %d seconds", t.ImageId, t.Limit),
				nil)
		}

		if t.Context.Debug {
			fmt.Println("Checking image status...")
		}

		resp, err := client.DescribeImages(context.TODO(), &ec2.DescribeImagesInput{
			ImageIds: []string{t.ImageId},
		})
		if err != nil || len(resp.Images) == 0 {
			if t.Context.Debug {
				fmt.Printf("Failed to get image status: %s\n", err)
			}
			time.Sleep(15 * time.Second)
			continue
		}

		if resp.Images[0].State == "available" {
			return t.Context.Result(true, fmt.Sprintf("AWS AMI %s available", t.ImageId), nil)
		}

		if t.Context.Debug {
			fmt.Printf("Image status is %s, sleeping for 15 seconds...\n", resp.Images[0].State)
		}
		time.Sleep(15 * time.Second)
	}
}
