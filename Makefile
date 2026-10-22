# Copyright (c) 2025-2026 Tenebris Technologies Inc.
# This software is licensed under the MIT License (see LICENSE for details).

BINARY := opsblade
MODULE := github.com/OpsBlade/OpsBlade
APP_PKG := $(MODULE)/app

GIT_COMMIT=$(shell git rev-parse --short=8 HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date +%FT%T%z)
GO_VERSION=$(shell go version | awk '{print $$3}')
# BUILD_NUMBER orders builds; GIT_COMMIT identifies their source. The commit
# cannot answer "is the copy I am running newer than the one I just built?" —
# a hash has no order — so a rebuild of the same commit is indistinguishable
# without this. UTC, because local time repeats an hour twice a year and a
# newer build would sort older. Assigned with := so one `make build-all`
# stamps ONE number across every platform: with a recursive `=` the shell
# re-runs per expansion and each target would land a second or two apart.
BUILD_NUMBER:=$(shell date -u +%Y%m%d%H%M%S)
LDFLAGS=-ldflags "-X $(APP_PKG).gitCommit=$(GIT_COMMIT) -X $(APP_PKG).buildTime=$(BUILD_TIME) -X $(APP_PKG).goVersion=$(GO_VERSION) -X $(APP_PKG).buildNumber=$(BUILD_NUMBER) -s -w"

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all build build-all check test vet clean install

# The default is test-then-build: binaries are produced only when the whole
# suite has passed. The recursive make keeps the order under -j.
all: test
	@$(MAKE) --no-print-directory build

build:
	go build $(LDFLAGS) -o $(BINARY) .

build-all:
	@mkdir -p bin
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		echo "building bin/$(BINARY)-$$os-$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o bin/$(BINARY)-$$os-$$arch . || exit 1; \
	done

# test is the one gate, for developers and CI alike: the full regression
# suite (build, vet, race-enabled tests), exit non-zero on any failure.
test:
	./test.sh

# check is kept as an alias for existing pipelines
check: test

vet:
	go vet ./...

clean:
	rm -f $(BINARY)
	rm -rf bin
	go clean -testcache

# install copies the freshly built binary: /usr/local/bin as root, ~/bin
# otherwise. It never creates ~/bin.
install: build
	@if [ "$$(id -u)" -eq 0 ]; then dest=/usr/local/bin; \
	elif [ -d "$$HOME/bin" ]; then dest="$$HOME/bin"; \
	else echo "make install: nowhere to install. A system install needs 'sudo make install'; a personal install needs ~/bin to exist." >&2; exit 1; fi; \
	install -m 0755 $(BINARY) "$$dest/$(BINARY)" && echo "installed $$dest/$(BINARY)"
