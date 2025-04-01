// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package refresh

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context          shared.TaskContext      `yaml:"context" json:"context"`                 // Task context
	Credentials      shared.Credentials      `yaml:"credentials" json:"credentials"`         // Allow override of credentials
	LaunchTemplateId string                  `yaml:"launch_template" json:"launch_template"` // Launch Template ID located ASGs
	SkipMatching     string                  `yaml:"skip_matching" json:"skip_matching"`     // Skip instances that match the launch template
	Filters          []shared.Filter         `yaml:"filters" json:"filters"`                 // Filters to pass to AWS API
	Select           []shared.SelectCriteria `yaml:"select" json:"select"`                   // Selection criteria to apply to the list of AMIs
	Fields           []string                `yaml:"fields" json:"fields"`                   // List of fields to return as data
}

func init() {
	shared.RegisterTask("aws_asg_refresh", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {

	if err := json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	if t.LaunchTemplateId == "" {
		return t.Context.Error("launch_template is required", nil)
	}

	// SkipMatching defaults to true
	skipMatching := true
	if t.SkipMatching != "" {
		value, err := strconv.ParseBool(t.SkipMatching)
		if err == nil {
			skipMatching = value
		} else {
			return t.Context.Error("failed to parse skip_matching", err)
		}
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

	asgClient := amazonInstance.ASGClient()

	// The first task is to obtain a list of autoscaling groups that
	// match the provided filters and the launch template
	req := &autoscaling.DescribeAutoScalingGroupsInput{
		Filters: cloudaws.FiltersToASG(t.Filters)}

	// Set up the paginator
	var asgList []string
	paginator := autoscaling.NewDescribeAutoScalingGroupsPaginator(asgClient, req,
		func(o *autoscaling.DescribeAutoScalingGroupsPaginatorOptions) {
			o.Limit = 25
			o.StopOnDuplicateToken = true
		})

	// Get the pages
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return t.Context.Error("error describing autoscaling groups", err)
		}

		// Iterate over the results and apply selection criteria
		var selected bool
		for _, asg := range page.AutoScalingGroups {

			// First create if LaunchTemplate exists and has a LaunchTemplateId
			if asg.LaunchTemplate == nil || asg.LaunchTemplate.LaunchTemplateId == nil {
				continue
			}

			if *asg.MinSize == 0 && *asg.DesiredCapacity == 0 {
				continue
			}

			// LaunchTemplate ID must match
			if strings.ToLower(*asg.LaunchTemplate.LaunchTemplateId) != strings.ToLower(t.LaunchTemplateId) {
				continue
			}

			// Optional selection criteria
			if len(t.Select) > 0 {
				selected, err = shared.ApplySelectionCriteria(asg, t.Select)
				if err != nil {
					return t.Context.Error("failed applying selection criteria", err)
				}

				if selected {
					asgList = append(asgList, *asg.AutoScalingGroupName)
				}
			} else {
				asgList = append(asgList, *asg.AutoScalingGroupName)
			}
		}
	}

	if len(asgList) == 0 {
		return t.Context.Error("no autoscaling groups matched the specified criteria", nil)
	}

	if t.Context.Debug {
		fmt.Printf("Found %d autoscaling groups to refresh: %v\n", len(asgList), asgList)
	}

	if t.Context.DryRun {
		return t.Context.Result(true, "Dry run, not refreshing autoscaling groups", asgList)
	}

	// Set up the results map
	asgResults := make(map[string]string)
	success := true

	// Iterate over the list of autoscaling groups and refresh them
	for _, asg := range asgList {
		_, err = asgClient.StartInstanceRefresh(context.TODO(),
			&autoscaling.StartInstanceRefreshInput{
				AutoScalingGroupName: &asg,
				Preferences: &types.RefreshPreferences{
					SkipMatching: aws.Bool(skipMatching),
					AutoRollback: aws.Bool(false),

					// AWS Defaults
					ScaleInProtectedInstances: "Wait",
					StandbyInstances:          "Wait",

					// This implements the "launch before terminating" policy
					MaxHealthyPercentage: aws.Int32(110),
					MinHealthyPercentage: aws.Int32(100),

					// Give instances time to warm up
					InstanceWarmup: aws.Int32(300),
				},
			})
		if err != nil {
			asgResults[asg] = fmt.Sprintf("Instance Refresh failed: %s", err.Error())
			success = false
		} else {
			asgResults[asg] = "success"
		}
	}

	return t.Context.Result(
		success,
		"AWS Autoscaling Group list",
		asgResults)
}
