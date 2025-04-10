// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

func SelectEnv(taskEnv, globalEnv string) string {
	if taskEnv != "" {
		return taskEnv
	}
	return globalEnv
}
