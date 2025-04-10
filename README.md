# OpsBlade

OpsBlade is an Ansible-inspired cloud operations tool written by a guy who got tired of maintaining a large number of scripts. After all, why spend an hour writing a script when you can spend a week writing a workflow framework?

**This tool is a work in progress. Use at your own risk. Please see the license and warranty (or more precisely the complete lack of any warranty) sections below.**

OneBlade executes tasks in sequence from a YAML file. A sample is included as example.yaml.txt. 

Each task is coded as a separate Go package in the workflow directory. Please refer to workflow/example/example.go for an example of how tasks work and how to add new ones.

## **ATTENTION: BREAKING CONFIG CHANGE in 0.1.8**

OpsBlade 0.1.8 has a significant change in the configuration subsystem. While the previous system was flexible, in retrospect allowing users to load configuration information from the yaml file or variables was a mistake. It made it too easy for users who version control their yaml files to accidentally commit credentials to a repository and it was not possible to ensure that credentials did not appear in debug output.

Credentials have now been entirely removed from the configuration file and replaced with "env:" at both the file and task level. If "env" is specified at the task level, the file it points to will be loaded into the environment by the appropriate service module. If "env" is not specified at the task level, the file level "env" (if specified) will be loaded.

Some service modules (AWS for example) will fall back to their default configuration files if no environment file is specified. Others, such as Slack and Jira will return an error.

Please see below for a list of supported environment variables.

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
go build -o opsblade
```

Users who intend to compile and run on different computers may wish to set CGO_ENABLED=0 to avoid reliance on the system's C libraries.

```
CGO_ENABLED=0 go build -o opsblade
```
And like most programs written in Go, cross-compilation using GOOS and GOARCH is available.

## Use

The yaml file consists of some global settings and a list of tasks. The task `name` is arbitrary and are intended for human use only. The `task` field is matched against the task registry and therefore must match a task identifier of an included module from workflow/. If the task identifier is not found, a fatal error occurs.

The following environment variables are supported:

### AWS

* AWS_ACCESS_KEY_ID
* AWS_SECRET_ACCESS_KEY
* AWS_REGION

### Slack

* SLACK_WEBHOOK

### Jira

* JIRA_USER
* JIRA_TOKEN
* JIRA_URL

## Copyright and license

Copyright (c) 2025 by Tenebris Technologies Inc. This software is licensed under the MIT License. Please see LICENSE for details.

## No Warranty (nada, zilch, nil, null)

THIS SOFTWARE IS PROVIDED “AS IS,” WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
