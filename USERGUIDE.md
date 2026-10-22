# OpsBlade User Guide

This guide is the reference for writing and running OpsBlade workflows. The
README covers installation and the headline behaviour changes; everything
about the YAML format, task behaviour, failure handling, notifications, and
credentials is here.

Contents

- 1. Running a workflow
- 2. Workflow file structure
- 3. Common task fields
- 4. Variables
- 5. Filters, select, and fields
- 6. Failure handling and exit codes
- 7. Notifications and transcripts
- 8. Credentials and environment files
- 9. Dry run
- 10. Task reference
- 11. A complete example
- 12. Troubleshooting
- 13. Using OpsBlade as a Go library

## 1. Running a workflow

```
opsblade [filename.yaml] [--stdin] [--json] [--dryrun] [--debug] [--version]
```

| Flag | Short | Effect |
|---|---|---|
| `--dryrun` | `-d` | Tasks that change resources simulate the change instead. Notifications are printed instead of sent. Section 9. |
| `--debug` | `-v` | Each task prints its resolved configuration before running, and some tasks print extra progress. |
| `--json` | `-j` | Task start and completion records are written as JSON instead of text. |
| `--stdin` | `-s` | Read the workflow from stdin instead of a file. Cannot be combined with a filename. |
| `--version` | `-V` | Print the version, build time, and toolchain, then exit. |

A flag on the command line is a default; the same setting in the workflow
file overrides it. So `opsblade -d workflow.yaml` runs dry unless the file
says `dryrun: false`.

### 1.1 What is written where

- **stderr**: the startup banner (name, version, copyright), and any error
  from sending a notification.
- **stdout**: one record when each task starts, one when it completes, and a
  final summary line.

Text records look like this:

```
* Starting task 2: "Load missing file" [variables_load]

* Completed task 2: "Load missing file" [variables_load]
Success: false
Outcome: warning
Message: failed to open file: open /nonexistent/file.json: no such file or directory

Expected on first run.
 (continuing: on_fail is warn)
Data: none
```

`Outcome` is one of `success`, `warning`, `fatal`, `stop`, or `skipped`
(section 6). `Data` lists the variables the task produced, as YAML. With
`--debug` the start record also prints the task's instructions and the task
prints its resolved configuration.

With `--json` each record is a JSON object. Start records have
`message_type: task_start` and carry `sequence`, `name`, `task`, and
`instructions`. Completion records have `message_type: task_stop` and carry
`sequence`, `name`, `task`, `success`, `outcome`, `msg`, `data`, and `stop`
(true when the task asked for a clean stop). Skipped tasks produce a
completion record with `message_type: task_skipped`. The summary line is
plain text in both modes.

The summary line is always the last line on stdout:

| Outcome | Summary |
|---|---|
| Completed | `All tasks complete. Exiting with code 0.` |
| Completed with warnings | `Completed with N warning(s). Exiting with code 0.` |
| Stopped | `Workflow stopped at task N "name" [type]: message. Exiting with code 0.` |
| Fatal | `Terminating due to fatal error in task N "name" [type]. Exiting with code 1.` |

### 1.2 Running from cron

Notifications (section 7) replace the usual practice of mailing stdout.
Leave stderr unredirected so cron still mails a failure to send the
notification itself. A typical entry:

```
15 2 * * * /usr/local/bin/opsblade /etc/opsblade/nightly.yaml > /dev/null
```

## 2. Workflow file structure

```yaml
name: Nightly staging deploy   # optional; used in notification subjects, defaults to the file name
dryrun: false                  # optional; overrides --dryrun
debug: false                   # optional; overrides --debug
json: false                    # optional; overrides --json
env: /home/user/.env           # optional; environment file loaded by tasks that need credentials

notify:                        # optional; see section 7
  slack: { env_suffix: _ALERTS }
  email: { to: [ops@example.com], from: "OpsBlade <opsblade@example.com>" }

tasks:
  - name: Human-readable name
    task: task_identifier
    # task-specific fields
```

Tasks run in order. Each task's output data becomes variables available to
later tasks (section 4). The workflow ends when the last task completes, when
a task fails fatally, or when a task stops it cleanly.

A workflow read from stdin with no `name` is called "OpsBlade workflow".

The file is checked before any task runs. An unknown task type, an invalid
`on_fail`, or an invalid `notify.email.transcript` value aborts the run with
exit code 1 and nothing executed.

## 3. Common task fields

Every task accepts these fields. Task-specific fields are in section 10.

| Field | Required | Meaning |
|---|---|---|
| `task` | yes | The task identifier. Must match a registered task exactly. |
| `name` | no | Free text shown in output and notifications. |
| `skip` | no | `true` skips the task. It is reported as skipped and counts as run. |
| `on_fail` | no | What a failure means: `fatal` (default), `warn`, or `stop`. Section 6. |
| `error_message` | no | Text appended to the task's message when it fails. Use it to explain expected failures. It appears in the console record and in notifications. |
| `env` | no | Environment file for this task, overriding the workflow's `env`. Only tasks that need credentials read it. |

`error_message` example:

```yaml
- name: Load staging deployment results
  task: variables_load
  filename: /data/pending.json
  error_message: "This is normal when the previous deployment succeeds. The file is created only when there are pending tasks."
  on_fail: stop
```

## 4. Variables

Variables are a single flat map of name to value shared by every task in a
run. They come from three places:

- `variables_set` and `variables_load` tasks.
- The `data` section of every task result. Each key becomes a variable of the
  same name, replacing any earlier value. Section 10 lists what each task
  produces.
- Three built-ins that resolve at use time: `{{date}}` (YYYYMMDD),
  `{{datetime}}` (YYYYMMDDhhmmss), and `{{epoch}}` (Unix seconds).

### 4.1 Referencing variables

Write `{{name}}` anywhere inside a task's field values, including inside
lists and maps such as `filters`, `select`, `tags`, and `set`. Substitution
happens when the task runs, so a variable set by task 2 is available to task
3 but not to task 1.

- A name that is not set resolves to an empty string. There is no error.
- A list value is substituted as its elements joined with commas.
- Other non-string values are converted to text.
- Nested paths such as `{{instance_data.0.InstanceId}}` are not supported in
  substitution. Use `fields` (section 5.3) on the producing task to surface
  the value you need, or `select` on `exit_if` to test nested data.

```yaml
- name: Create the AMI
  task: aws_ec2_ami_create
  instance_id: "{{instance_id}}"
  instance_name: "web-{{date}}"
  tags:
    Source: "{{instance_id}}"
```

Quote string values in YAML so `"yes"`, `"1"`, and similar are not
reinterpreted as booleans or numbers.

### 4.2 Persisting variables between runs

`variables_save` writes the variable map, or selected fields of it, to a
JSON file. `variables_load` reads such a file back. This is how state is
carried from one run or one workflow to another, for example a list of
instances patched last night that a morning workflow must verify. The file is
a plain JSON object and may also be written by other tools.

## 5. Filters, select, and fields

Three list fields shape what a task asks for and returns. Which of them a
task honours is stated in section 10.

### 5.1 filters

Passed to the cloud API as the request filter, so the API does the work. For
AWS tasks these are the filter names documented for the corresponding
Describe call, each with a list of acceptable values:

```yaml
filters:
  - name: tag:Environment
    values: ["staging"]
  - name: instance-state-name
    values: ["running", "stopped"]
```

### 5.2 select

Applied by OpsBlade to each item the API returned. Every criterion must
match for the item to be kept. If `select` is empty everything is kept.

```yaml
select:
  - field: State.Name
    value: "running"
    compare: equal
  - field: Tags.*.Value
    value: "web"
    compare: begins
  - field: LaunchTime
    value: 30
    compare: days_old
```

Field paths:

- Use dot notation to walk into nested structures. Names are matched
  case-insensitively, so `state.name` and `State.Name` are the same.
- `*` steps into a list and tries each element; the criterion matches if any
  element matches.
- A path that reaches a list without `*` also tries each element.
- A path that does not exist never matches.

Operators:

| compare | Meaning |
|---|---|
| `equal` | Equal. Case-insensitive for strings. |
| `not` | Not equal. Case-insensitive for strings. |
| `contains` | String contains the value, case-insensitive. |
| `begins` | String begins with the value, case-insensitive. |
| `greater` | Greater than. Numeric for numbers, lexical for strings. |
| `less` | Less than. Numeric for numbers, lexical for strings. |
| `after` | The field and the value are both parsed as dates; the field is later. |
| `before` | The field and the value are both parsed as dates; the field is earlier. |
| `days_old` | The field is parsed as a date and is at least this many days in the past. |
| `minutes_old` | The field is parsed as a date and is at least this many minutes in the past. |

Value typing matters. The item is compared using the type of the field as
it appears in JSON, and the `value` must be the same kind:

- String fields need a string `value`. Quote it. `equal`, `not`,
  `contains`, `begins`, `greater`, `less`, `after`, `before`, `days_old`,
  and `minutes_old` apply.
- Numeric fields need an unquoted numeric `value`. `equal`, `not`,
  `greater`, and `less` apply.
- Boolean fields need `true` or `false`. `equal` and `not` apply.
- `days_old` and `minutes_old` accept a number or a numeric string.

A type mismatch, such as a quoted `"30"` against a numeric field, never
matches; it is not an error.

Dates are recognised in these forms: RFC 3339 with or without fractional
seconds (`2026-09-11T02:01:48Z`, `2026-09-11T02:01:48-04:00`),
`2026-09-11T02:01:48`, `2026-09-11`, `20260911020148`, and `20260911`.
AWS timestamps are RFC 3339. A field that does not parse never matches.

`exit_if` applies `select` to the variable map rather than to API results,
which is how a workflow tests its own state. Top-level variables are the
first path element, so `instance_count` or `instance_data.*.State.Name`.

### 5.3 fields

Chooses which parts of each returned item become task data. When `fields`
is empty the whole item is returned. Dot notation walks into nested
structures and `*` iterates a list, applying the rest of the path to each
element:

```yaml
fields:
  - InstanceId
  - State.Name
  - Tags.*.Value
```

Rules:

- Names are matched case-insensitively. Output keys keep the item's own
  spelling.
- When a path crosses a list with `*`, the remainder is flattened into a
  key, so `Tags.*.Value` yields a list under `Value`, and `a.*.b.c` yields
  values keyed `b.c`.
- If a field and one of its children are both listed, the child is dropped
  and the whole field is returned.
- A path that does not exist is silently absent from the output.

Field names for AWS items are the AWS SDK names as they appear in the API
response, for example `InstanceId`, `ImageId`, `State.Name`, `LaunchTime`,
`Tags`, `AutoScalingGroupName`, `GroupId`. Run the list task once with no
`fields` and `--debug` to see what is available.

`variables_save`, `variables_load`, and `variables_dump` apply `fields` to
the variable map itself, so paths there start with a variable name:
`instance_data.*.InstanceId`.

## 6. Failure handling and exit codes

Every task has an `on_fail` field that says what a failure of that task
means. The default is `fatal`.

| Value | Meaning |
|---|---|
| `fatal` | The failure aborts the workflow. Remaining tasks do not run. Exit code 1. A FATAL ERROR alert is sent if notifications are configured. |
| `warn` | The failure is recorded and the workflow continues. Exit code 0 if nothing else fails. A single WARNING alert listing every warned task is sent at the end. |
| `stop` | The failure is a normal condition, not an error. The workflow stops cleanly. Exit code 0. No alert. |

```yaml
- name: Announce start
  task: slack_send
  subject: "Deploy starting"
  on_fail: warn        # a deleted channel must not stop the deployment

- name: Load pending deployment
  task: variables_load
  filename: /data/pending.json
  error_message: "Normal when nothing is pending."
  on_fail: stop        # nothing to do; exit 0 without an alert

- name: Refresh ASG
  task: aws_asg_refresh
  # on_fail: fatal is the default
```

Guidance:

- A step that changes a resource (patching an image, refreshing an ASG,
  creating an issue) is `fatal`.
- A step that gathers data later steps depend on is `fatal`.
- A notification is `warn`.
- Loading optional state that may legitimately be absent is `stop` if the
  workflow has nothing to do without it, `warn` if it should proceed with
  defaults.
- `exit_if` produces a clean stop by itself and needs no `on_fail`.
- `cmd_exec` still accepts `no_fail: true`, which is equivalent to
  `on_fail: warn`.

An invalid `on_fail` value is detected before any task runs, so a typo
cannot cause a partial run.

A warning that occurs before a later clean stop is still a warning: the run
is reported as stopped, the WARNING alert is not sent, but the warned task
appears in the STOPPED transcript email if one is configured.

Some tasks distinguish an expected condition from an error and classify the
two separately. `jira_issue_check` uses `on_mismatch` for "the issue is not
in the required state" and `on_fail` for "the check could not be performed".
See section 10.4.

### 6.1 Exit codes

This table is authoritative.

| Outcome | Exit code | Alert |
|---|---|---|
| All tasks succeeded | 0 | none |
| Completed, one or more `warn` failures | 0 | WARNING |
| Stopped by `on_fail: stop`, `on_mismatch: stop`, or `exit_if` | 0 | none |
| Aborted by a `fatal` failure | 1 | FATAL ERROR |
| Workflow could not load, or a task type is unknown | 1 | FATAL ERROR |

A YAML file that cannot be parsed exits 1 without any notification, because
the `notify` block is not available until the file has been parsed. With
`transcript: always` (section 7) a COMPLETED or STOPPED email is also sent
for the exit 0 outcomes that have no alert.

## 7. Notifications and transcripts

A workflow can send a loud alert when it completes with warnings or aborts,
and can email the full transcript of every run.

```yaml
notify:
  slack:
    env: /etc/opsblade/alerts.env   # optional; credentials file for this channel only
    env_suffix: _ALERTS             # optional; reads SLACK_WEBHOOK_ALERTS instead of SLACK_WEBHOOK
    mention: channel                # optional; channel (default), here, or none
  email:
    env: /etc/opsblade/smtp.env     # optional
    to: [ops@example.com]
    from: "OpsBlade <opsblade@example.com>"
    subject_prefix: "[OpsBlade]"    # optional; applied to every email
    transcript: always              # optional; never, error, or always (default)
```

- `from` and each `to` entry may be a bare address or `Name <address>`.
  The name appears in the mail client; the bare address is used on the SMTP
  envelope. Quote the form with a name so YAML does not misread the angle
  brackets. Without a name, some mail clients show the address twice, as
  `opsblade@example.com <opsblade@example.com>`. An address that does not
  parse is reported as `invalid from address` or `invalid recipient address`
  when the email is sent, and nothing is delivered.
- Include `slack`, `email`, or both. When both are present every alert goes
  to both, independently. A Slack failure does not suppress the email; it is
  reported on stderr and noted in the email body above the transcript. An
  email failure is reported on stderr only.
- Notifications are sent after the workflow finishes. Failures to send are
  printed to stderr and do not change the exit code.
- In dry-run mode the notification is printed instead of sent, without the
  transcript, followed by a line saying the transcript was omitted.
- Each channel accepts its own `env:` if its credentials live in a different
  file than the workflow's global `env`.

### 7.1 Which notification is sent

| Outcome | Slack | Email, `transcript: never` | `error` | `always` (default) |
|---|---|---|---|---|
| Completed | none | none | none | COMPLETED + transcript |
| Stopped | none | none | none | STOPPED + transcript |
| Warnings | WARNING | WARNING | WARNING + transcript | WARNING + transcript |
| Fatal | FATAL ERROR | FATAL ERROR | FATAL ERROR + transcript | FATAL ERROR + transcript |

Subjects are `<prefix> <SEVERITY>: <workflow name>`, for example
`[OpsBlade] FATAL ERROR: Nightly staging deploy` or
`[OpsBlade] COMPLETED: Nightly staging deploy`.

### 7.2 Alert content

Both channels carry the severity, workflow name, host, and start time, then
one entry per task that warned, aborted, or stopped the workflow. Each entry
is a severity label followed by aligned lines: the task number and type, its
name, what it reported including any `error_message`, and the action taken.
A FATAL ERROR alert ends with a CAUTION line when tasks were left unrun.
Task data and variables are never included.

```
FATAL ERROR: Nightly staging deploy
Host: ops1
Started: 2026-09-11T01:40:46-04:00

WARNING:
    Task:   1 - slack_send
    Name:   "Announce"
    Error:  failed to send Slack message: non-200 response from Slack: 404 Not Found (channel_not_found)
    Action: continued

FATAL ERROR:
    Task:   3 - aws_asg_refresh
    Name:   "Refresh ASG"
    Error:  failed to start refresh: ...
    Action: aborted

CAUTION: 2 task(s) did not run.
```

The `Name:` line is omitted for a task with no name. A STOPPED entry uses
`Reason:` in place of `Error:` and `Action: stopped`. A multi-line message,
such as one with an `error_message`, continues indented under the value
column.

The Slack message starts with `<!channel>` or `<!here>` according to
`mention`, followed by a warning or rotating-light icon, and shows each
entry in a code block under a bold label.

### 7.3 Transcript

OpsBlade captures everything it writes to stdout and stderr, while still
writing it to the terminal, so the run's output can be emailed without
redirecting it to a file. The `transcript` setting under `email` controls
its use:

| Value | Effect |
|---|---|
| `always` (default) | Every run sends an email. WARNING and FATAL ERROR alerts carry the transcript after a separator block. Successful and stopped runs send a COMPLETED or STOPPED email with the same header and the transcript. |
| `error` | Only WARNING and FATAL ERROR alerts are emailed, with the transcript appended. |
| `never` | Only WARNING and FATAL ERROR alerts are emailed, without the transcript. |

The transcript starts after command-line parsing, so it includes the
banner, every task record, debug output, and the exit summary line. It is
appended after a separator:

```
CAUTION: 2 task(s) did not run.

----------------------------------------------------------------------
Transcript
----------------------------------------------------------------------
OpsBlade 0.2.00+5962ddf4 [20260911060147]
Copyright (c) 2025-2026 Tenebris Technologies Inc.

* Starting task 1: "Announce" [slack_send]
...
Terminating due to fatal error in task 3 "Refresh ASG" [aws_asg_refresh]. Exiting with code 1.
```

Errors from sending the notification itself happen after capture and
appear only on stderr. Slack never receives the transcript. There is no
size limit; a `--debug` run with large API responses produces a large
email. An invalid `transcript` value is rejected when the file loads,
before any task runs.

## 8. Credentials and environment files

Credentials are never placed in the workflow file. They come from
environment files in `.env` format (`NAME=value`, one per line, `#`
comments allowed), named by the workflow `env` field or a task's own `env`
field. A task-level `env` wins. Tasks that need no credentials ignore both.
Values already present in the process environment are not overwritten by
the file, so a variable exported before running OpsBlade takes precedence.

| Service | Variables | Notes |
|---|---|---|
| AWS | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` | See 8.1. `AWS_ACCOUNT_ID` is optional and is the default `account_id` for `aws_account_check` (section 10.5). |
| Jira | `JIRA_USER`, `JIRA_TOKEN`, `JIRA_URL` | All three are required. `JIRA_USER` is the account email, `JIRA_TOKEN` an API token, `JIRA_URL` the site base URL such as `https://example.atlassian.net/`. |
| Slack | `SLACK_WEBHOOK` | An incoming webhook URL. An `env_suffix` on `slack_send` or `notify.slack` appends to the name, so `_DEV` reads `SLACK_WEBHOOK_DEV`. |
| Email | `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS` | Notifications only. `SMTP_HOST` is required. Port defaults to 587; 465 selects implicit TLS, other ports use STARTTLS when the server offers it. `SMTP_USER` enables PLAIN authentication with `SMTP_PASS`. The `from` and `to` addresses themselves are set in the `notify.email` block (section 7), not in the environment. |

### 8.1 AWS credential resolution

For each AWS task, in order:

1. If the task has a `profile` field, that named profile from
   `~/.aws/config` and `~/.aws/credentials` is used.
2. Otherwise, if both `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` are
   set, they are used as static credentials with `AWS_REGION`.
3. Otherwise the SDK default chain applies: environment, shared config,
   instance or container role.

A task's `region` field overrides the region from any of these sources.
A missing region is an error at the first API call.

Set `dryrun: true` at the file level to verify that credentials load as
expected without changing anything (section 9).

## 9. Dry run

`--dryrun` on the command line or `dryrun: true` in the file runs the
workflow without changing resources. It is the recommended way to check
credentials, filters, and variable flow before a first real run. Not every
task behaves the same way in dry run:

| Task | Dry-run behaviour |
|---|---|
| `variables_set`, `variables_load`, `variables_dump`, `exit_if`, `dryrun_or_die` | Run normally. |
| `variables_save`, `file_delete` | Report what they would do; nothing is written or deleted. |
| `sleep` | Does not sleep. |
| `cmd_exec` | Command is not executed. `cmd_output` is not set. |
| `slack_send` | No message is sent. |
| `jira_issue_create` | Assignee and sprint lookups run for real; no issue is created. `jira_issue_id` is set to `jira-issue-dry-run`. |
| `jira_issue_check`, `aws_account_check` | Run for real. They are read-only. |
| `jira_issue_comment`, `jira_issue_attach_file` | **Run for real.** These tasks do not check dry run. Use `skip: true` while testing. |
| `aws_ec2_instance_start`, `aws_ec2_instance_stop`, `aws_ec2_ami_create`, `aws_ec2_lt_change_image` | The AWS DryRun flag is sent, so AWS checks permissions and parameters without acting. `aws_ec2_ami_create` sets `image_id` to `AMI-none-dry-run`. |
| `aws_ec2_ami_wait`, `aws_ec2_instance_wait` | Return immediately. |
| `aws_asg_refresh` | Lists matching groups for real, then reports `success` for each without starting a refresh. |
| `aws_ec2_instance_list`, `aws_ec2_ami_list`, `aws_ec2_sg_list`, `aws_asg_list`, `aws_asg_describe_refreshes` | Run normally. They are read-only, so a dry run sees the same data as a real run and later tasks receive their real variables. |

## 10. Task reference

Every task also accepts the common fields in section 3. "Produces" lists the
variables the task sets on success. Dry-run behaviour is in section 9.

### 10.1 Variables and files

#### variables_set

Sets one or more variables.

| Field | Meaning |
|---|---|
| `set` | List of `{name, value}` pairs. Values may be any YAML type, including lists and maps. |

Produces: one variable per entry.

```yaml
- name: Set variables
  task: variables_set
  set:
    - name: environment
      value: "staging"
    - name: instance_ids
      value: ["i-0abc", "i-0def"]
```

#### variables_load

Loads variables from a JSON file written by `variables_save` or by hand.

| Field | Meaning |
|---|---|
| `filename` | Path to read. A missing or unreadable file, or invalid JSON, is a failure; use `on_fail: stop` or `warn` if that is expected. |
| `fields` | Optional. Load only these fields (section 5.3). |

Produces: every loaded key.

#### variables_save

Writes the current variables to a JSON file.

| Field | Meaning |
|---|---|
| `filename` | Path to write. Overwritten if present. |
| `fields` | Optional. Save only these fields, with wildcard support. |

Produces: nothing new. The saved data is shown in the task record but not
re-applied as variables.

#### variables_dump

Attaches the current variables, or a selection, to the task record. Useful
for seeing what earlier tasks produced.

| Field | Meaning |
|---|---|
| `fields` | Optional. Limit the output. |

Produces: the selected variables again, unchanged.

#### file_delete

Deletes a file.

| Field | Meaning |
|---|---|
| `filename` | Path to delete. Required. A missing file is a failure. |

### 10.2 Control and utility

#### exit_if

Stops the workflow cleanly when the current variables match the `select`
criteria. This is the normal way to end a run early. It is not a failure and
needs no `on_fail`.

| Field | Meaning |
|---|---|
| `select` | Criteria evaluated against the variable map (section 5.2). |

Produces: `exit_if_result` (`true` when the criteria matched).

```yaml
- name: Nothing to do
  task: exit_if
  select:
    - field: instance_count
      value: 0
      compare: equal
```

#### sleep

Pauses the workflow.

| Field | Meaning |
|---|---|
| `sleep` | Seconds to wait. |

#### dryrun_or_die

Fails unless the workflow is running in dry-run mode. Place it first in a
workflow that must never run for real from a given invocation, for example
a file used only to verify credentials.

#### cmd_exec

Runs a local command and captures its combined stdout and stderr.

| Field | Meaning |
|---|---|
| `cmd` | Program to run. Resolved via PATH. No shell is involved, so quoting, globbing, and redirection do not apply; use `sh -c` if needed. |
| `args` | List of arguments. |
| `no_fail` | Deprecated. `true` is equivalent to `on_fail: warn`. |

Produces: `cmd`, `cmd_args`, `cmd_output`. A non-zero exit is a failure.
Output is captured, not streamed, so a long-running command shows nothing
until it finishes.

#### example

A template task that returns mock data. Accepts `filters`, `select`, and
`fields` to demonstrate them and produces `mock_data`. See
`workflow/example/example.go` for how tasks are written.

### 10.3 Slack

#### slack_send

Posts a message to a Slack incoming webhook.

| Field | Meaning |
|---|---|
| `subject` | Bold first line. |
| `body` | Message text. Slack mrkdwn is honoured. |
| `pretty` | Optional list of variable names. Each is appended to the message, pretty-printed as YAML in a code block. Useful for `instance_data` or `asg_refresh_results`. |
| `env_suffix` | Optional. Selects `SLACK_WEBHOOK<suffix>`. |

Produces: `slack_subject`, `slack_body`. A missing webhook or a non-200
response is a failure. Use `on_fail: warn` so a broken channel does not
abort real work.

### 10.4 Jira

All Jira tasks need `JIRA_USER`, `JIRA_TOKEN`, and `JIRA_URL`.

#### jira_issue_create

Creates an issue.

| Field | Meaning |
|---|---|
| `project` | Project key. |
| `issue_type` | Issue type name, for example `Task`. |
| `summary` | Issue title. |
| `description` | Issue body. Text of the form `[user@example.com]` is replaced, on a best-effort basis, with a mention of that user's account. |
| `assignee` | Optional email address. Resolved to an account ID; an unknown address is a failure. Empty means the project default. |
| `active_sprint` | Optional. `true` finds the project's board and active sprint and adds the issue to it. |
| `fields` | Optional selection of the created issue's fields to return. |

Produces: `jira_issue_id` (the key, for example `OPS-123`), `jira_project`,
`jira_assignee`, `jira_assignee_account_id`, plus any selected fields.

#### jira_issue_comment

Adds a comment. Runs for real in dry run.

| Field | Meaning |
|---|---|
| `issue_id` | Issue key. Required. |
| `comment` | Comment text. Required. |

#### jira_issue_attach_file

Attaches a local file. Runs for real in dry run.

| Field | Meaning |
|---|---|
| `issue_id` | Issue key. Required. |
| `file_name` | Path to the file. Required. |

#### jira_issue_check

Fetches an issue and compares its status and resolution with the required
values, case-insensitively. Either requirement may be omitted. The task
distinguishes two different things that can go wrong:

- **The issue is not in the required state.** This is an expected condition,
  not an error. It is classified by `on_mismatch`.
- **The check could not be performed.** Missing credentials, an HTTP error
  such as 401 or 404, or a network failure is an error and is classified by
  `on_fail`.

| Field | Meaning |
|---|---|
| `issue_id` | Issue key. Required. |
| `required_status` | Status name that must match, for example `Done / Closed`. |
| `required_resolution` | Resolution name that must match, for example `Done`. An unresolved issue has resolution `none`. |
| `on_mismatch` | What a mismatch means: `stop` (default), `warn`, or `fatal`. Same meanings as `on_fail`. |

```yaml
- name: Check that the change ticket is approved
  task: jira_issue_check
  issue_id: "{{jira_issue_id}}"
  required_status: "Done / Closed"
  required_resolution: "Done"
  error_message: "The change ticket is not approved yet."
  # on_mismatch: stop  is the default: not approved yet, exit 0, no alert
  # on_fail: fatal     is the default: Jira error, exit 1, FATAL ERROR alert
```

The separation matters because a workflow that waits for a ticket to be
approved should exit quietly when it is not approved yet, but must not exit
quietly when a token has expired or Jira is down. `error_message` is
appended in both cases.

Produces, whenever the issue was fetched, including on a mismatch:
`check_jira_issue_id`, `check_jira_issue_required_status`,
`check_jira_issue_required_resolution`, `check_jira_issue_status`,
`check_jira_issue_resolution`, and `check_jira_issue_passed` (`true` or
`false`). Nothing is set when the fetch failed. An invalid `on_mismatch`
value is reported when the task runs and is treated as an error under
`on_fail`.

With `on_mismatch: warn` the workflow continues; guard later tasks with
`exit_if` on `check_jira_issue_passed` if they must not run on a mismatch.

### 10.5 AWS account and EC2

All AWS tasks accept `env`, `region`, and `profile` (section 8.1). List
tasks accept `filters`, `select`, and `fields` (section 5) and return each
kept item as a map of the AWS response fields, reduced by `fields`.

#### aws_account_check

Confirms that the credentials resolve to the expected AWS account before
anything is changed. It asks STS for the caller identity using the same
`env`, `region`, and `profile` resolution as every other AWS task, so it
verifies exactly the credentials the following AWS tasks will use. A wrong
account is otherwise only visible as a confusing "does not exist" error on
the first resource lookup, or not at all.

| Field | Meaning |
|---|---|
| `account_id` | The 12-digit account the credentials must belong to. Optional when `AWS_ACCOUNT_ID` is set in the environment file, which keeps the account next to the credentials it describes. The field wins when both are set. |
| `on_mismatch` | `fatal` (default), `warn`, or `stop`. Applies when the identity was fetched but the account differs. API and credential errors use `on_fail` as usual. |

Produces: `aws_account_id`, `aws_arn`, `aws_user_id`,
`check_aws_account_passed` (`true` or `false`). On a mismatch the variables
are still set so a later task can inspect the outcome.

```yaml
- name: Confirm we are in the production account
  task: aws_account_check
  env: prod.env
  account_id: "123456789012"
```

With `AWS_ACCOUNT_ID=123456789012` in `prod.env`, the `account_id` line can
be omitted.

#### aws_ec2_instance_list

Describes instances.

| Field | Meaning |
|---|---|
| `owner` | Optional. Adds an `owner-id` filter; `self` is common. |
| `filters`, `select`, `fields` | Section 5. |

Produces: `instance_data` (list of instances), `instance_count`.

```yaml
- name: Find running web servers
  task: aws_ec2_instance_list
  filters:
    - name: tag:Role
      values: ["web"]
    - name: instance-state-name
      values: ["running"]
  fields:
    - InstanceId
    - PrivateIpAddress
```

#### aws_ec2_instance_start

Starts an instance.

| Field | Meaning |
|---|---|
| `instance_id` | Instance to start. |

Produces: `instance_id`.

#### aws_ec2_instance_stop

Stops an instance.

| Field | Meaning |
|---|---|
| `instance_id` | Instance to stop. |
| `force` | Optional. Force stop. |

Produces: `instance_id`.

#### aws_ec2_instance_wait

Polls until an instance reaches a state. When the state is `running` it also
waits for both the system and instance status checks to pass, so the
instance is usable when the task completes.

| Field | Meaning |
|---|---|
| `instance_id` | Instance to watch. |
| `state` | `running`, `stopped`, or `terminated`. |
| `limit` | Maximum seconds to wait. Exceeding it is a failure. |

#### aws_ec2_ami_list

Describes AMIs.

| Field | Meaning |
|---|---|
| `owner` | Optional. Owner filter; `self` is common and avoids listing public images. |
| `filters`, `select`, `fields` | Section 5. |

Produces: `ami_data`, `ami_count`.

#### aws_ec2_ami_create

Creates an AMI from an instance.

| Field | Meaning |
|---|---|
| `instance_id` | Source instance. |
| `instance_name` | Name of the new AMI. |
| `description` | AMI description. |
| `tags` | Map of tag name to value applied to the AMI. |
| `no_reboot` | Optional. `true` skips the reboot before imaging. The default reboot gives a consistent filesystem. |

Produces: `image_id`. Creation is asynchronous; follow with
`aws_ec2_ami_wait` before using the image.

#### aws_ec2_ami_wait

Polls every 15 seconds until an AMI is `available`.

| Field | Meaning |
|---|---|
| `image_id` | AMI to watch. |
| `limit` | Maximum seconds to wait. Exceeding it is a failure. |

#### aws_ec2_lt_change_image

Creates a new launch template version from the current default version with
a different AMI, and makes the new version the default.

| Field | Meaning |
|---|---|
| `lt_id` | Launch template ID. |
| `image_id` | AMI for the new version. |
| `fields` | Optional selection of the new version's fields to return; empty returns all. |

Produces: `launch_template`.

#### aws_ec2_sg_list

Describes security groups.

| Field | Meaning |
|---|---|
| `filters`, `select`, `fields` | Section 5. |

Produces: `security_group_data`, `security_group_count`.

### 10.6 AWS Auto Scaling

#### aws_asg_list

Describes auto scaling groups.

| Field | Meaning |
|---|---|
| `filters`, `select`, `fields` | Section 5. |

Produces: `asg_data`, `asg_count`.

#### aws_asg_refresh

Starts an instance refresh on every auto scaling group that uses one of the
given launch templates, directly or through a mixed instances policy, and
passes the filters and select criteria. The refresh launches before
terminating (minimum 100% healthy, maximum 110%), waits for scale-in
protected and standby instances, uses a 300 second warmup, and does not
auto-rollback.

| Field | Meaning |
|---|---|
| `launch_templates` | List of launch template IDs. At least one is required. |
| `skip_matching` | Optional string `"true"` or `"false"`, default true. Skip instances already on the current template version. |
| `filters`, `select` | Narrow the groups. |

Produces: `asg_refresh_count`, `asg_refresh_results` (map of group name to
`success` or the error text). If any refresh fails the task fails after
attempting the rest. Matching no groups is a failure. The task returns as
soon as the refreshes are started; use `aws_asg_describe_refreshes` to
follow progress.

#### aws_asg_describe_refreshes

Describes instance refreshes for one or more groups.

| Field | Meaning |
|---|---|
| `asg_name` | A single group. |
| `asgs` | A list of groups. |
| `most_recent` | Optional. Return only the newest refresh per group. |
| `filters`, `select`, `fields` | Section 5. Useful fields include `Status`, `PercentageComplete`, `StartTime`, `EndTime`. |

Produces: `describe_refreshes`, `describe_refreshes_count`.

## 11. A complete example

A nightly workflow that patches a golden instance, images it, rolls the new
image out to an auto scaling group, and records what it did.

```yaml
name: Nightly web image refresh
env: /etc/opsblade/prod.env

notify:
  slack:
    env_suffix: _ALERTS
  email:
    to: ["Ops Team <ops@example.com>"]
    from: "OpsBlade <opsblade@example.com>"
    subject_prefix: "[OpsBlade]"
    transcript: always

tasks:
  - name: Load the golden instance record
    task: variables_load
    filename: /var/lib/opsblade/golden.json      # sets golden_instance_id, web_lt_id

  - name: Confirm the change ticket is approved
    task: jira_issue_check
    issue_id: "OPS-42"
    required_status: "Approved"
    error_message: "Change not approved; nothing was changed."
    # on_mismatch: stop (default) exits quietly; a Jira outage is fatal and alerts

  - name: Announce
    task: slack_send
    subject: "Web image refresh starting"
    body: "Patching {{golden_instance_id}}"
    on_fail: warn

  - name: Start the golden instance
    task: aws_ec2_instance_start
    instance_id: "{{golden_instance_id}}"

  - name: Wait for it to be usable
    task: aws_ec2_instance_wait
    instance_id: "{{golden_instance_id}}"
    state: running
    limit: 600

  - name: Patch it
    task: cmd_exec
    cmd: /usr/local/bin/patch-golden
    args: ["{{golden_instance_id}}"]

  - name: Stop it
    task: aws_ec2_instance_stop
    instance_id: "{{golden_instance_id}}"

  - name: Wait for it to stop
    task: aws_ec2_instance_wait
    instance_id: "{{golden_instance_id}}"
    state: stopped
    limit: 600

  - name: Create the AMI
    task: aws_ec2_ami_create
    instance_id: "{{golden_instance_id}}"
    instance_name: "web-{{date}}"
    description: "Nightly patched web image"
    tags:
      Role: web
      Built: "{{datetime}}"

  - name: Wait for the AMI
    task: aws_ec2_ami_wait
    image_id: "{{image_id}}"
    limit: 1800

  - name: Point the launch template at it
    task: aws_ec2_lt_change_image
    lt_id: "{{web_lt_id}}"
    image_id: "{{image_id}}"

  - name: Roll it out
    task: aws_asg_refresh
    launch_templates: ["{{web_lt_id}}"]

  - name: Record the run
    task: variables_save
    filename: "/var/lib/opsblade/last-refresh.json"
    fields: [image_id, asg_refresh_results]
    on_fail: warn

  - name: Done
    task: slack_send
    subject: "Web image refresh complete"
    body: "New image {{image_id}}"
    pretty: [asg_refresh_results]
    on_fail: warn
```

If the ticket is not approved the run stops at task 2 with exit 0 and a
STOPPED email carrying the transcript. If patching fails at task 6 the run
aborts with exit 1, a FATAL ERROR alert to Slack and email listing the
failed task and noting that 8 tasks did not run, and the transcript in the
email. If Slack is broken, tasks 3 and 14 warn, the run completes with exit
0, and a WARNING alert goes to email.

## 12. Troubleshooting

**`Invalid task: xyz`** before anything runs. The `task` value does not
match a registered identifier. Check spelling against section 10.

**`on_fail must be "warn", "fatal", or "stop"`** or **`invalid notify
block`**. A typo in a control field. Nothing ran.

**`failed to create AWS client`**, **`missing required Jira configuration`**,
**`webhook is not configured`**. The environment file was not found or lacks
the variables in section 8. Confirm the `env` path is absolute and readable
by the user running OpsBlade, and that a task-level `env` is not pointing
elsewhere.

**A variable is empty.** Substitution of an unset name yields an empty
string silently. Add a `variables_dump` task before the point of use to see
what is set, or run with `--debug` to see each task's resolved
configuration. Remember that `fields` on the producing task limits what is
returned.

**`select` keeps nothing.** Check the value type (section 5.2): a quoted
number against a numeric field, or a date the parser does not recognise,
never matches. Run the list task once without `select` and look at the
data in the task record.

**No email arrived.** Run with `--dryrun` to confirm the `notify` block
parsed and see the body that would be sent. An `invalid from address` or
`invalid recipient address` error on stderr means an entry in the `notify.email`
block is not a valid address; the accepted forms are `user@example.com` and
`"Name <user@example.com>"`. Check stderr from the real run
for `email notification failed`. Remember that `transcript: error` and
`never` send nothing on a successful run.

**The Slack alert arrived but not the email**, or the reverse. The channels
are independent; the failure of the other is on stderr, and a Slack failure
is also noted in the email body.

## 13. Using OpsBlade as a Go library

`main.go` is a thin wrapper around the `workflow`, `notify`, and
`transcript` packages and serves as the example.

```go
capture, _ := transcript.Start()          // optional
w := workflow.New(workflow.WithDryRun(true), workflow.WithCallback(cb))
if err := w.Load("deploy.yaml"); err != nil { ... }
result := w.Execute()
fmt.Println(result.Summary())

alert := w.Alert(result)                  // built for every outcome
if capture != nil {
    alert.Transcript = capture.Stop()
}
errs := notify.New(w.Notify, notify.WithGlobalEnv(w.Env)).Send(alert)
os.Exit(result.ExitCode())
```

- `workflow.New` accepts `WithDryRun`, `WithDebug`, `WithJSON`, and
  `WithCallback`. Values in the loaded file override the first three.
- `Load` reads a file, or stdin when given an empty name, defaults the
  name, and validates the `notify` block. `AddTask`, `AddTaskJSON`, and
  `AddTaskYAML` build a workflow in code from a map, JSON, or YAML task.
- `Execute` validates every `on_fail`, runs the tasks, and returns a
  `Result` with `Outcome` (`completed`, `warnings`, `stopped`, `fatal`),
  `Warnings`, `Fatal`, `StoppedAt`, `TasksRun`, `TasksTotal`, and
  `Started`. It never sends notifications and never exits the process.
- A `shared.Callback` receives `OnStart(TaskInfo)` and `OnStop(TaskResult)`
  for each task instead of the console output being printed. Returning
  false from `OnStop` halts the workflow and is treated as fatal unless the
  task had already stopped or aborted it. The return value of `OnStart` is
  ignored.
- `Alert` builds a notification for any outcome. `notify.Send` decides what
  to deliver: Slack for WARNING and FATAL ERROR only, email according to the
  `transcript` setting. `notify.WithDryRun` prints instead of sending;
  `notify.WithOutput` redirects that print. `WithSlackSender` and
  `WithEmailSender` substitute senders for tests.
- `transcript.Start` replaces `os.Stdout` and `os.Stderr` with pipes that
  tee to the originals and a buffer; `Stop` restores them and returns the
  text. It captures only the standard streams, not a logger whose writer
  was captured before `Start` was called.
- A task result may set its own `OnFail` to override the task's setting for
  that result. This is how a task reports an expected condition (a mismatch,
  a missing optional file) differently from an error while leaving the
  engine's classification unchanged. Tasks request a clean stop with
  `TaskContext.Stop`.
- Variables are process-global in the `shared` package. Two workflows run in
  the same process share them.
