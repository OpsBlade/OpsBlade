// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

// Callback defines an interface that calling processes can optionally use to receive task start and result information
type Callback interface {
	OnStart(info TaskInfo) bool
	OnStop(result TaskResult) bool
}
