// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package wait

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context     shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials `yaml:"credentials" json:"credentials"` // Allow override of credentials
	InstanceId  string             `yaml:"instance_id" json:"instance_id"` // Instance ID to create AMI from
	State       string             `yaml:"state" json:"state"`             // Desired state of the instance
	Limit       int                `yaml:"limit" json:"limit"`             // Maximum number of seconds to wait
}

func init() {
	shared.RegisterTask("aws_ec2_instance_wait", func(context shared.TaskContext) shared.Task {
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

	// Force state to lowercase
	t.State = strings.ToLower(t.State)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	if t.State != "running" && t.State != "stopped" && t.State != "terminated" {
		return t.Context.Error(fmt.Sprintf("invalid state %s", t.State), nil)
	}

	// Resolve credentials with priority to task credentials, then context credentials
	creds := shared.NewCredentials(t.Credentials, *t.Context.Credentials)
	amazonInstance, err := cloudaws.New(
		cloudaws.WithRegion(creds.AWS.Region),
		cloudaws.WithAccessKey(creds.AWS.AccessKey),
		cloudaws.WithSecretKey(creds.AWS.SecretKey),
		cloudaws.WithProfile(creds.AWS.Profile),
		cloudaws.WithConfigFile(creds.AWS.ConfigFile),
		cloudaws.WithCredsFile(creds.AWS.CredsFile))
	if err != nil || amazonInstance == nil {
		return t.Context.Error("failed to create AWS client", err)
	}

	client := amazonInstance.EC2Client()
	startTime := time.Now()

	// First wait for the instance to reach the desired state
	if t.Context.Debug {
		fmt.Printf("Waiting for instance %s to reach state %s...\n", t.InstanceId, t.State)
	}

	if t.Context.DryRun {
		return t.Context.Result(true, "DryRun, no instance state change", nil)
	}

	// Wait until the instance reaches the desired state or the time limit is reached
	for {
		if time.Since(startTime) > time.Duration(t.Limit)*time.Second {
			return t.Context.Error(
				fmt.Sprintf("timeout, instance %s not in state %s after %d seconds", t.InstanceId, t.State, t.Limit),
				nil)
		}

		resp, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
			InstanceIds: []string{t.InstanceId},
		})
		if err != nil || len(resp.Reservations) == 0 {
			if t.Context.Debug {
				fmt.Printf("Failed to describe instance: %v\n", err)
			}
			time.Sleep(10 * time.Second)
			continue
		}

		if len(resp.Reservations[0].Instances) > 0 {
			currentState := string(resp.Reservations[0].Instances[0].State.Name)
			if strings.ToLower(currentState) == t.State {

				// If we only care about stopped or terminated, we're done
				if t.State != "running" {
					return t.Context.Result(true, fmt.Sprintf("instance %s is in state %s", t.InstanceId, t.State), nil)
				}

				// For running state, we need to also check status checks
				break
			}

			if t.Context.Debug {
				fmt.Printf("instance state is '%s', waiting for '%s'...\n", currentState, t.State)
			}
		}
		time.Sleep(10 * time.Second)
	}

	// If the desired state is "running", wait for status checks to pass
	if t.State == "running" {

		if t.Context.Debug {
			fmt.Printf("Waiting for instance %s to pass status checks...\n", t.InstanceId)
		}

		// Wait until the instance passes status checks or the time limit is reached
		for {
			if time.Since(startTime) > time.Duration(t.Limit)*time.Second {
				return t.Context.Error(
					fmt.Sprintf("timeout, instance %s status checks did not pass after %d seconds", t.InstanceId, t.Limit),
					nil)
			}

			statusResp, err := client.DescribeInstanceStatus(context.TODO(), &ec2.DescribeInstanceStatusInput{
				InstanceIds:         []string{t.InstanceId},
				IncludeAllInstances: aws.Bool(true),
			})

			if err != nil || len(statusResp.InstanceStatuses) == 0 {
				if t.Context.Debug {
					fmt.Printf("Failed to get instance status: %v\n", err)
				}
				time.Sleep(10 * time.Second)
				continue
			}

			status := statusResp.InstanceStatuses[0]
			systemStatus := string(status.SystemStatus.Status)
			instanceStatus := string(status.InstanceStatus.Status)

			if systemStatus == "ok" && instanceStatus == "ok" {
				if t.Context.Debug {
					fmt.Printf("Instance %s passed all status checks\n\n", t.InstanceId)
				}
				return t.Context.Result(true,
					fmt.Sprintf("instance %s is running and passed all status checks", t.InstanceId),
					map[string]any{"instance_id": t.InstanceId})
			}

			if t.Context.Debug {
				fmt.Printf("System status: '%s', Instance status: '%s', waiting...\n", systemStatus, instanceStatus)
			}
			time.Sleep(10 * time.Second)
		}
	}

	return t.Context.Result(true, fmt.Sprintf("instance %s is in state %s", t.InstanceId, t.State),
		map[string]any{"instance_id": t.InstanceId})
}
