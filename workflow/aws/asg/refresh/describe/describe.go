// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package describe

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context    shared.TaskContext      `yaml:"context" json:"context"`         // Task context
	Env        string                  `yaml:"env" json:"env"`                 // Optional file to load into the environment
	Region     string                  `yaml:"region" json:"region"`           // AWS region - allow overriding
	Profile    string                  `yaml:"profile" json:"profile"`         // AWS profile - allow overriding
	ASGName    string                  `yaml:"asg_name" json:"asg_name"`       // Name of the autoscaling group
	ASGList    []string                `yaml:"asgs" json:"asgs"`               // List of autoscaling groups
	MostRecent bool                    `yaml:"most_recent" json:"most_recent"` // Return only the most recent refresh
	Filters    []shared.Filter         `yaml:"filters" json:"filters"`         // Filters to pass to AWS API
	Select     []shared.SelectCriteria `yaml:"select" json:"select"`           // Selection criteria to apply to the list of AMIs
	Fields     []string                `yaml:"fields" json:"fields"`           // List of fields to return as data
}

func init() {
	shared.RegisterTask("aws_asg_describe_refreshes", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// If a single name is specified, just append it to the list
	if t.ASGName != "" {
		t.ASGList = append(t.ASGList, t.ASGName)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
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

	var data []any
	for _, ASGName := range t.ASGList {

		req := &autoscaling.DescribeInstanceRefreshesInput{
			AutoScalingGroupName: aws.String(ASGName)}

		// Set up the paginator
		paginator := autoscaling.NewDescribeInstanceRefreshesPaginator(asgClient, req,
			func(o *autoscaling.DescribeInstanceRefreshesPaginatorOptions) {
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
			for _, refresh := range page.InstanceRefreshes {
				if len(t.Select) > 0 {
					selected, err = shared.ApplySelectionCriteria(refresh, t.Select)
					if err != nil {
						return t.Context.Error("failed applying selection criteria", err)
					}

					if selected {
						data = append(data, shared.SelectFields(refresh, t.Fields))
					}
				} else {
					data = append(data, shared.SelectFields(refresh, t.Fields))
				}

				// If most_recent is set, we only want the first item, whether selected or not
				if t.MostRecent {
					break
				}
			}

		}
	}

	rData := make(map[string]any)
	rData["describe_refreshes"] = data
	rData["describe_refreshes_count"] = len(data)
	return t.Context.Result(
		true,
		"AWS ASG Describe Refreshes",
		rData)
}
