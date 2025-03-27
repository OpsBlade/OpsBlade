# OpsBlade

OpsBlade is an Ansible-inspired cloud operations tool written by a guy who got tired of maintaining a large number of scripts. After all, why spend an hour writing a script when you can spend a week writing a workflow framework?

**This tool is a work in progress. Use at your own risk. Please see the license and warranty (or more precisely the complete lack of any warranty) sections below.**

OneBlade executes tasks in sequence from a YAML file. A sample is included as example.yaml.txt. 

Each task is coded as a separate Go package in the workflow directory. Please refer to workflow/example/example.go for an example of how tasks work and how to add new ones.

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

## Copyright and license

Copyright (c) 2025 by Tenebris Technologies Inc. This software is licensed under the MIT License. Please see LICENSE for details.

## No Warranty (nada, zilch, nil, null)

THIS SOFTWARE IS PROVIDED “AS IS,” WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE, AND NON-INFRINGEMENT. IN NO EVENT SHALL THE COPYRIGHT HOLDERS OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
