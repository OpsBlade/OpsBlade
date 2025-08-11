# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OpsBlade is an Ansible-inspired cloud operations tool written in Go. It executes tasks sequentially from YAML configuration files, designed for AWS, Jira, and Slack operations automation.

## Development Commands

### Build
```bash
# Standard build
go build -o opsblade

# Build without CGO dependencies (for portability)
CGO_ENABLED=0 go build -o opsblade

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o opsblade-linux
```

### Test
```bash
# Run all tests
go test ./...

# Run tests in a specific package
go test ./shared/

# Run with verbose output
go test -v ./...
```

### Run
```bash
# Execute a workflow from file
./opsblade workflow.yaml

# Execute with flags
./opsblade -d workflow.yaml  # Dry run mode
./opsblade -v workflow.yaml  # Debug mode
./opsblade -j workflow.yaml  # JSON output
./opsblade -s < workflow.yaml  # Read from stdin
```

## Architecture

### Core Structure

The project follows a modular task-based architecture:

1. **Main Entry Point** (`main.go`): CLI interface that loads and executes workflows
2. **Workflow Engine** (`workflow/workflow.go`): Orchestrates task execution from YAML configurations
3. **Task System**: Each operation is implemented as a self-registering task module

### Task Registration Pattern

Tasks self-register via `init()` functions when imported in `workflow/workflow.go`. Each task:
- Implements the `shared.Task` interface with an `Execute()` method
- Registers itself with `shared.RegisterTask(taskID, constructor)`
- Deserializes its own configuration from `Context.Instructions`

### Key Components

- **services/**: Cloud service clients (AWS, Jira, Slack)
  - `cloudaws/`: AWS SDK wrappers for EC2, ASG operations
  - `cloudjira/`: Jira API client for issue management
  - `cloudslack/`: Slack webhook integration
  
- **shared/**: Common utilities and interfaces
  - `TaskContext`: Core task execution context
  - `TaskResult`: Standardized result handling
  - Variable resolution system (`{{var_name}}` syntax)
  - Field filtering and selection utilities

- **workflow/**: Task implementations organized by service
  - `aws/`: EC2, ASG, AMI management tasks
  - `jira/`: Issue creation, commenting, attachments
  - `slack/`: Message sending
  - `variables/`: Variable management (set, save, load)
  - `misc/`: Utility tasks (sleep, dryrun, exitIf)

### Configuration Flow

1. Global settings (dryrun, debug, json, env) loaded from YAML
2. Task-specific environment files override global settings
3. Variables resolved using `{{variable_name}}` syntax
4. Filters, select criteria, and field selections applied to API results

### Credential Management

Credentials are loaded from environment files (.env format), never from YAML:
- AWS: Uses SDK default chain or AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
- Jira: Requires JIRA_USER, JIRA_TOKEN, JIRA_URL
- Slack: Uses SLACK_WEBHOOK (with optional env_suffix for multiple channels)

## Adding New Tasks

1. Create a new package under `workflow/[service]/[operation]/`
2. Implement the task struct with `Context shared.TaskContext`
3. Add an `init()` function to register the task
4. Implement `Execute()` method returning `shared.TaskResult`
5. Import the package in `workflow/workflow.go` using underscore import

Example structure from `workflow/example/example.go` provides a complete template.