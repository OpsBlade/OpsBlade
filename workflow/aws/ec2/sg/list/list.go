package list

import (
	"context"
	"encoding/json"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type Task struct {
	Context shared.TaskContext      `yaml:"context" json:"context"` // Task context
	Env     string                  `yaml:"env" json:"env"`         // Optional file to load into the environment
	Region  string                  `yaml:"region" json:"region"`   // AWS region - allow overriding
	Profile string                  `yaml:"profile" json:"profile"` // AWS profile allow overriding
	Filters []shared.Filter         `yaml:"filters" json:"filters"` // Filters to pass to AWS API
	Select  []shared.SelectCriteria `yaml:"select" json:"select"`   // Selection criteria to apply to the list of AMIs
	Fields  []string                `yaml:"fields" json:"fields"`   // List of fields to return as data
}

func init() {
	shared.RegisterTask("aws_ec2_sg_list", func(context shared.TaskContext) shared.Task {
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

	// Resolve environment file
	envFile := shared.SelectEnv(t.Env, t.Context.Env)

	amazonInstance, err := cloudaws.New(
		cloudaws.WithRegion(t.Region),
		cloudaws.WithEnvironment(envFile),
		cloudaws.WithProfile(t.Profile))
	if err != nil || amazonInstance == nil {
		return t.Context.Error("failed to create AWS client", err)
	}

	client := amazonInstance.EC2Client()
	req := &ec2.DescribeSecurityGroupsInput{
		Filters: cloudaws.FiltersToEC2(t.Filters),
		DryRun:  &t.Context.DryRun}

	// Set up the paginator
	var sgData []any
	paginator := ec2.NewDescribeSecurityGroupsPaginator(client, req,
		func(o *ec2.DescribeSecurityGroupsPaginatorOptions) {
			o.Limit = 25
			o.StopOnDuplicateToken = true
		})

	// Get the pages
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return t.Context.Error("error describing instances", err)
		}

		// Iterate over the results and apply selection criteria
		for _, sg := range page.SecurityGroups {
			if len(t.Select) > 0 {
				selected, err := shared.ApplySelectionCriteria(sg, t.Select)
				if err != nil {
					return t.Context.Error("failed applying selection criteria", err)
				}

				if selected {
					sgData = append(sgData, shared.SelectFields(sg, t.Fields))
				}
			} else {
				sgData = append(sgData, shared.SelectFields(sg, t.Fields))
			}
		}

	}

	data := make(map[string]any)
	data["security_group_data"] = sgData
	data["security_group_count"] = len(sgData)
	return t.Context.Result(
		true,
		"AWS EC2 Security Group list",
		data)
}
