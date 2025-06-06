// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package start

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"strconv"
	"strings"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

type Task struct {
	Context         shared.TaskContext      `yaml:"context" json:"context"`                   // Task context
	Env             string                  `yaml:"env" json:"env"`                           // Optional file to load into the environment
	Region          string                  `yaml:"region" json:"region"`                     // AWS region - allow overriding
	Profile         string                  `yaml:"profile" json:"profile"`                   // AWS profile - allow overriding
	LaunchTemplates []string                `yaml:"launch_templates" json:"launch_templates"` // Launch Template ID located ASGs
	SkipMatching    string                  `yaml:"skip_matching" json:"skip_matching"`       // Skip instances that match the launch template
	Filters         []shared.Filter         `yaml:"filters" json:"filters"`                   // Filters to pass to AWS API
	Select          []shared.SelectCriteria `yaml:"select" json:"select"`                     // Selection criteria to apply to the list of AMIs
	Fields          []string                `yaml:"fields" json:"fields"`                     // List of fields to return as data
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

	if len(t.LaunchTemplates) < 1 {
		return t.Context.Error("at least one launch template must be specified", nil)
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

	// Resolve environment file
	envFile := shared.SelectEnv(t.Env, t.Context.Env)

	amazonInstance, err := cloudaws.New(
		cloudaws.WithRegion(t.Region),
		cloudaws.WithEnvironment(envFile),
		cloudaws.WithProfile(t.Profile))
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
			o.Limit = 10
			o.StopOnDuplicateToken = true
		})

	// Get the pages
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return t.Context.Error("error describing autoscaling groups", err)
		}

		var selected bool
		for _, asg := range page.AutoScalingGroups {

			// First check if LaunchTemplate exists and has a LaunchTemplates
			id := ""
			if asg.LaunchTemplate == nil || asg.LaunchTemplate.LaunchTemplateId == nil {
				// Check for one in MixedInstancesPolicy
				if asg.MixedInstancesPolicy == nil || asg.MixedInstancesPolicy.LaunchTemplate == nil {
					// No launch template, skip
					continue
				} else {
					// Check if the LaunchTemplateId is in the list
					if asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification == nil ||
						asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification.LaunchTemplateId == nil {
						continue
					}
					id = *asg.MixedInstancesPolicy.LaunchTemplate.LaunchTemplateSpecification.LaunchTemplateId
				}
			} else {
				id = *asg.LaunchTemplate.LaunchTemplateId
			}

			if id == "" {
				continue
			}

			if *asg.MinSize == 0 && *asg.DesiredCapacity == 0 {
				continue
			}

			// LaunchTemplate ID must match one item in the list
			if !foundInList(t.LaunchTemplates, id) {
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
		fmt.Printf("Found %d autoscaling groups to refresh:\n", len(asgList))
		for _, item := range asgList {
			fmt.Printf("  %s\n", item)
		}
		fmt.Println("")
	}

	// Set up the results map
	asgResults := make(map[string]string)
	success := true

	// Iterate over the list of autoscaling groups and refresh them
	for _, asg := range asgList {

		if t.Context.DryRun {
			asgResults[asg] = "success"
		} else {

			// Refresh the instances in the ASG
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
	}

	var msg string
	if t.Context.DryRun {
		msg = "Dry run, ASG refresh simulated"
	} else {
		msg = "AWS Autoscaling Groups refreshed"
	}
	return t.Context.Result(
		success,
		msg,
		map[string]any{"asg_refresh_count": len(asgList), "asg_refresh_results": asgResults})
}

func foundInList(list []string, item string) bool {
	for _, i := range list {
		if strings.ToLower(i) == strings.ToLower(item) {
			return true
		}
	}
	return false
}
