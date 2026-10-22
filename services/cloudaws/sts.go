// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudaws

import (
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// STSClient returns an STS client
func (c *CloudAWS) STSClient() *sts.Client {
	return sts.NewFromConfig(*c.AWS)
}
