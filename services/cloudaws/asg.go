// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudaws

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling/types"

	"github.com/OpsBlade/OpsBlade/shared"
)

// ASGClient returns an auto scaling group client
func (c *CloudAWS) ASGClient() *autoscaling.Client {
	return autoscaling.NewFromConfig(*c.AWS)
}

func FiltersToASG(filters []shared.Filter) []types.Filter {
	var awsFilters []types.Filter
	for _, filter := range filters {
		awsFilters = append(awsFilters, types.Filter{Name: aws.String(filter.Name), Values: filter.Values})
	}
	return awsFilters
}
