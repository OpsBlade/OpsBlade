// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudaws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/OpsBlade/OpsBlade/shared"
)

// EC2Client returns an EC2 client
func (c *CloudAWS) EC2Client() *ec2.Client {
	return ec2.NewFromConfig(*c.AWS)
}

func FiltersToEC2(filters []shared.Filter) []types.Filter {
	var awsFilters []types.Filter
	for _, filter := range filters {
		awsFilters = append(awsFilters, types.Filter{Name: aws.String(filter.Name), Values: filter.Values})
	}
	return awsFilters
}
