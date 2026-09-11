# OpsBlade

OpsBlade is an Ansible-inspired cloud operations tool written by a guy who got tired of maintaining a large number of scripts. After all, why spend an hour writing a script when you can spend a week writing a workflow framework?

**This tool is a work in progress. Use at your own risk. Please see the license and warranty (or more precisely the complete lack of any warranty) sections below.**

OneBlade executes tasks in sequence from a YAML file. A sample is included as example.yaml.txt. 

Each task is coded as a separate Go package in the workflow directory. Please refer to workflow/example/example.go for an example of how tasks work and how to add new ones.

## **ATTENTION: BEHAVIOUR CHANGE: failure handling and exit codes**

Task failures are now classified by a per-task `on_fail` field (`warn`, `fatal`, or `stop`; default `fatal`), and a workflow can send a WARNING or FATAL ERROR alert to Slack and/or email. See USERGUIDE.md sections 6 and 7.

Two things changed that existing cron jobs and wrapper scripts may depend on:

* `exit_if`, and any task with `on_fail: stop`, now end the workflow with **exit code 0**. Previously a deliberate stop exited 1 and printed "Terminating due to failed task".
* The startup banner (name, version, copyright) is now written to **stderr** so that stdout carries only task output.
* Alert bodies have a new layout: each entry is a severity label followed by the task number and type, its name, the error, and the action taken. The `notify.email` block gains a `transcript` option, default `always`, which emails the run's full output after every run. If you already email stdout from cron, set `transcript: error` or `never`, or drop the cron redirect.
* `jira_issue_check` no longer treats an issue that is not in the required state as an error. By default it now stops the workflow cleanly with **exit code 0** and no alert. Jira API and credential errors are still failures governed by `on_fail`. See USERGUIDE.md section 10.4.

## **ATTENTION: BREAKING CONFIG CHANGE in 0.1.8**

OpsBlade 0.1.8 has a significant change in the configuration subsystem. While the previous system was flexible, in retrospect allowing users to load configuration information from the yaml file or variables was a mistake. It made it too easy for users who version control their yaml files to accidentally commit credentials to a repository and it was not possible to ensure that credentials did not appear in debug output.

Credentials have now been entirely removed from the configuration file and replaced with "env:" at both the file and task level. If "env" is specified at the task level, the file it points to will be loaded into the environment by the appropriate service module. If "env" is not specified at the task level, the file level "env" (if specified) will be loaded.

Some service modules (AWS for example) will fall back to their default configuration files if no environment file is specified. Others, such as Slack and Jira will return an error.

See USERGUIDE.md section 8 for the supported environment variables.

Users may wish to set `dryrun: true` at the file level to verify that credentials are loaded as expected.

## Contributions

PRs and Issues are welcome, as is helping with testing and documentation. Please be patient, this tool is not how I earn living.

## Acknowledgements

Thanks to @Jacktheyeti for the naming suggestion.

## Building

OpsBlade is developed in Go. To build it, you will need to have Go installed on your system. If you don't already, you can download it for free from https://go.dev/dl/.

```
git clone https://github.com/OpsBlade/OpsBlade.git
cd OpsBlade
make build
```

`make build` stamps the binary with the git commit and build time; `opsblade --version` shows them. A plain `go build -o opsblade` also works and produces an unstamped binary. `make build-all` cross-compiles for Linux and macOS on amd64 and arm64 into `bin/`. `make check` runs the full regression suite (build, vet, race-enabled tests) and exits non-zero on any failure; it is the gate for CI/CD pipelines. `make test` and `./test.sh` are equivalent.

Users who intend to compile and run on different computers may wish to set CGO_ENABLED=0 to avoid reliance on the system's C libraries.

```
CGO_ENABLED=0 go build -o opsblade
```
And like most programs written in Go, cross-compilation using GOOS and GOARCH is available.

## Use

```
opsblade [filename.yaml] [--stdin] [--json] [--dryrun] [--debug] [--version]
```

A workflow is a YAML file with a few global settings and a list of tasks that run in order. `example.yaml.txt` is a commented sample. Each task's output becomes variables that later tasks can reference with `{{name}}`.

**The complete reference is [USERGUIDE.md](USERGUIDE.md).** It covers the file format, common task fields, variables, filters and selection, failure handling and exit codes, notifications and transcripts, credentials, every task's fields and outputs, and library use. In brief:

* Every task has `on_fail`: `fatal` (default, abort with exit 1 and a FATAL ERROR alert), `warn` (continue, WARNING alert at the end), or `stop` (a normal condition, exit 0, no alert). `exit_if` stops cleanly by itself. `jira_issue_check` uses `on_mismatch` for an issue that is not in the required state and `on_fail` for Jira errors.
* A top-level `notify` block sends WARNING and FATAL ERROR alerts to Slack and/or email. With `transcript: always` (the default) email also carries the run's full output, so cron no longer needs to mail stdout. Email addresses may carry a display name, as in `"OpsBlade <opsblade@example.com>"`.
* Credentials never go in the YAML file. They come from `.env` files named by `env:` at the file or task level: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` (or the SDK default chain); `JIRA_USER`, `JIRA_TOKEN`, `JIRA_URL`; `SLACK_WEBHOOK` with an optional suffix; `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS` for email notifications.

## Copyright and license

Copyright (c) 2025-2026 by Tenebris Technologies Inc. This software is licensed under the MIT License. Please see LICENSE for details.

## No Warranty (nada, zilch, nil, null)

THIS SOFTWARE IS PROVIDED “AS IS,” WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
