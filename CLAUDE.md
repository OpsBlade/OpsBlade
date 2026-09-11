# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

OpsBlade is an Ansible-inspired cloud operations tool written in Go. It executes tasks sequentially from YAML configuration files, designed for AWS, Jira, and Slack operations automation.

## Development Commands

### Build
```bash
# Standard build (stamps commit, build time, and build number via ldflags)
make build

# Cross-compile for linux/darwin on amd64/arm64 into bin/
make build-all

# Plain build without stamping
go build -o opsblade
```

### Test
```bash
# Full regression suite: build, vet, go test -race. This is the CI/CD gate; it exits non-zero on any failure.
make check         # equivalent: make test, ./test.sh
./test.sh -x       # keep the test log

# Single package
go test -race -count=1 ./workflow/
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

1. **Main Entry Point** (`main.go`): CLI interface that loads and executes workflows, then sends alerts
2. **Workflow Engine** (`workflow/workflow.go`): Orchestrates task execution from YAML configurations and returns a `Result`
3. **Task System**: Each operation is implemented as a self-registering task module
4. **Identity** (`app/`): Name, tagline, copyright, and version, reached only through accessors. Bump `version` there; nothing else defines it. Build metadata is injected by the Makefile ldflags.
5. **Notifications** (`notify/`): Workflow-level notifications driven by the `notify` block in the YAML. WARNING and FATAL ERROR alerts go to Slack and/or email; COMPLETED and STOPPED transcript emails go to email only, per `notify.email.transcript` (`never`, `error`, `always`; default `always`).
6. **Transcript** (`transcript/`): Captures stdout and stderr for the life of the process while passing them through, so `main.go` can attach the run's output to the notification. Started right after flag parsing; every exit path in `main.go` drains it via `exit()`.

### Failure Model

Each task result is classified by the engine using the task's `on_fail` field:

- `fatal` (default): abort, exit 1, FATAL ERROR alert
- `warn`: record and continue, exit 0, single WARNING alert at the end
- `stop`: clean stop, exit 0, no alert. Tasks can also request this directly via `TaskContext.Stop()`, which is how `exit_if` works.

A task can separate an expected condition from an error by setting `TaskResult.OnFail` on the result; the engine's `classify()` uses it in place of the task's `on_fail`. `jira_issue_check` does this: a status or resolution mismatch carries its `on_mismatch` value (`stop` default), while API and credential errors are ordinary failures under `on_fail`. Prefer this pattern over inventing new outcome kinds.

`on_fail` values and the `notify` block are validated before any task runs. `Execute()` returns a `workflow.Result` with the outcome, warnings, fatal task, stop task, and counts; `Result.ExitCode()` and `Result.Summary()` drive `main.go`. `Workflow.Alert()` builds a notification for every outcome; `notify.New(...).Send()` decides what to deliver (Slack for alerts only, email per `transcript`) and is called from `main.go`, never from inside the engine, so library users are not surprised by Slack traffic. The recorded failure message is the task's own; the "(continuing ...)" note is added to the console copy only. A `Callback` can still veto continuation; a veto is treated as fatal.

The exit-code table in USERGUIDE.md section 6.1 is authoritative. Keep it in sync with `Result.ExitCode()`.

### Task Registration Pattern

Tasks self-register via `init()` functions when imported in `workflow/workflow.go`. Each task:
- Implements the `shared.Task` interface with an `Execute()` method
- Registers itself with `shared.RegisterTask(taskID, constructor)`
- Deserializes its own configuration from `Context.Instructions`

### Key Components

- **services/**: Cloud service clients (AWS, Jira, Slack, email)
  - `cloudaws/`: AWS SDK wrappers for EC2, ASG operations
  - `cloudjira/`: Jira API client for issue management
  - `cloudslack/`: Slack webhook integration
  - `cloudemail/`: SMTP sender for notifications (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`). Addresses may be `Name <addr>`; the bare address goes on the envelope, the full form in the headers
  
- **transcript/**: stdout and stderr capture with pass-through, used only by `main.go`

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

1. Global settings (name, dryrun, debug, json, env, notify) loaded from YAML; `notify` is validated at load
2. Task-specific environment files override global settings
3. Variables resolved using `{{variable_name}}` syntax
4. Filters, select criteria, and field selections applied to API results

### Documentation

- `README.md` is the overview: build, behaviour-change notices, and a short "Use" summary that links to the guide.
- `USERGUIDE.md` is the user reference: file format, common fields, variables, filters and selection, failure handling, exit codes, notifications and transcripts, credentials, a per-task reference (fields and produced variables), and library use. Any user-visible change (a new task, field, variable, notification behaviour, or exit code) must be reflected there. Adding a task means adding its section 10 entry and its row in the dry-run table in section 9.
- `example.yaml.txt` is the commented sample and should show new fields once.
- `PLAN*.md` and `UPGRADE*.md` are gitignored working documents and must not be referenced from published docs.

### Layout Note

The `services/` and `workflow/` container directories predate the Go standards in `~/.claude/standards` and are left as is. New root-level concepts (`app/`, `notify/`) follow the standard. New service clients go under `services/` to match the existing family. New Go files use the project's existing two-line copyright header.

### Credential Management

Credentials are loaded from environment files (.env format), never from YAML:
- AWS: Uses SDK default chain or AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
- Jira: Requires JIRA_USER, JIRA_TOKEN, JIRA_URL
- Slack: Uses SLACK_WEBHOOK (with optional env_suffix for multiple channels)
- Email: SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS (notifications only)

## Adding New Tasks

1. Create a new package under `workflow/[service]/[operation]/`
2. Implement the task struct with `Context shared.TaskContext`
3. Add an `init()` function to register the task
4. Implement `Execute()` method returning `shared.TaskResult`
5. Import the package in `workflow/workflow.go` using underscore import

Example structure from `workflow/example/example.go` provides a complete template.