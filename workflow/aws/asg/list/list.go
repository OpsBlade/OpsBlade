// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package list

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/autoscaling"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context     shared.TaskContext      `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials      `yaml:"credentials" json:"credentials"` // Allow override of credentials
	Filters     []shared.Filter         `yaml:"filters" json:"filters"`         // Filters to pass to AWS API
	Select      []shared.SelectCriteria `yaml:"select" json:"select"`           // Selection criteria to apply to the list of AMIs
	Fields      []string                `yaml:"fields" json:"fields"`           // List of fields to return as data
}

func init() {
	shared.RegisterTask("aws_asg_list", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
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

	req := &autoscaling.DescribeAutoScalingGroupsInput{
		Filters: cloudaws.FiltersToASG(t.Filters)}

	// Set up the paginator
	var asgData []any
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
			if len(t.Select) > 0 {
				selected, err = shared.ApplySelectionCriteria(asg, t.Select)
				if err != nil {
					return t.Context.Error("failed applying selection criteria", err)
				}

				if selected {
					asgData = append(asgData, shared.SelectFields(asg, t.Fields))
				}
			} else {
				asgData = append(asgData, shared.SelectFields(asg, t.Fields))
			}
		}

	}

	data := make(map[string]any)
	data["asg_data"] = asgData
	data["asg_count"] = len(asgData)
	return t.Context.Result(
		true,
		"AWS Autoscaling Group list",
		data)
}
