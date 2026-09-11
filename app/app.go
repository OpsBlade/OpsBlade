// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

// Package app holds the application's identity — its name, tagline, copyright
// and version — and is the only place any of them is defined.
//
// Everything here is unexported and reached through an accessor. That is a
// deliberate reversal of the usual "export the constant" habit: an exported
// constant sitting beside an accessor gives two ways to read one value, and call
// sites will use both. That is not hypothetical — it produced a binary that
// rendered its own version two different ways depending on which symbol each
// site reached for.
//
// Build metadata is injected at link time. The version itself never is: it is a
// property of the source, not of the machine that compiled it, so it stays a
// const the linker cannot reach. Deriving a version from `git describe` produces
// a binary that disagrees with its own source.
package app

import (
	"runtime"
	"strconv"
)

const (
	name      = "OpsBlade"
	tagLine   = "Ansible-inspired cloud operations workflow runner"
	copyright = "Copyright (c) 2025-2026 Tenebris Technologies Inc."

	// version is the release number, bare semver. Bump it here; nothing else
	// defines a version. Build tooling reads this line, so keep it a single
	// assignment on one line.
	version = "0.2.00"
)

// Build metadata, injected via ldflags:
//
//	-X github.com/OpsBlade/OpsBlade/app.gitCommit=<sha8>
//	-X github.com/OpsBlade/OpsBlade/app.buildTime=<rfc3339>
//	-X github.com/OpsBlade/OpsBlade/app.goVersion=<go version>
//	-X github.com/OpsBlade/OpsBlade/app.buildNumber=<utc yyyymmddhhmmss>
//
// Unexported: `go tool link -X` sets package-level string vars by symbol name
// and does not care about case, so privacy costs nothing here. All four are
// empty under a plain `go build`, which is fine — they decorate the version,
// never replace it.
//
// Do NOT group these into a struct: -X cannot write a struct field, and fails
// silently with exit code 0, so every binary would ship a blank commit from a
// green build.
var (
	gitCommit   string
	buildTime   string
	goVersion   string
	buildNumber string
)

// Name returns the product name.
func Name() string { return name }

// TagLine returns the one-line product description.
func TagLine() string { return tagLine }

// Copyright returns the copyright notice.
func Copyright() string { return copyright }

// Version returns the build's full identity: the release number, the build
// commit as SemVer build metadata, and the build number in brackets —
// "0.1.0+1a2b3c4d [20260902155301]" — degrading to "0.1.0+1a2b3c4d" or bare
// "0.1.0" as those are stamped in.
//
// This is the form for anywhere a human or a model reads it: startup banner,
// version command, logs, diagnostics.
//
// The release and the commit are one unbroken token on purpose. Rendered as
// "0.1.0 (git: 1a2b3c4d)" it gets copied into a bug report as "0.1.0" — the
// space reads as the end of the value — so the half that identifies the exact
// source is the half that gets dropped. The build number is deliberately on the
// far side of the space: truncating there still leaves the commit attached, and
// the number answers a different question anyway (see Build).
//
// The separator is "+", not "-", because SemVer gives the two different
// meanings. "+" introduces build metadata, which the spec requires be IGNORED
// when comparing versions, so "0.1.0+1a2b3c4d" compares equal to "0.1.0" — it is
// that release, built from that commit. "-" would introduce a PRE-RELEASE
// identifier, making it compare LOWER than "0.1.0" and claiming to be something
// that came before the release rather than an instance of it.
func Version() string {
	v := version
	if gitCommit != "" {
		v += "+" + gitCommit
	}
	if buildNumber != "" {
		v += " [" + buildNumber + "]"
	}
	return v
}

// Build returns the build number: the UTC time the binary was linked, as
// yyyymmddhhmmss, or "" under a plain `go build`.
//
// It exists because the commit hash cannot answer "is the copy I am running
// newer than the one I built last?". A hash identifies source exactly but has no
// order, so telling two builds apart at a glance means looking each one up. This
// number only orders builds — it says nothing about which source they came from,
// which is the commit's job — but ordering is the whole point: 20260902155301
// is plainly later than 20260902094117.
//
// A timestamp rather than a counter because a counter needs stored state, and a
// commit count does not move when the same commit is rebuilt — exactly the case
// worth catching while iterating against a running install. Seconds rather than
// minutes because two rebuilds inside one minute is ordinary. UTC because local
// time runs backwards an hour twice a year, which would make a newer build look
// older.
func Build() string { return buildNumber }

// BuildDate returns the calendar day the binary was linked, as YYYYMMDD
// (for example 20260902), or 0 under a plain `go build`.
//
// Use this when an existing protocol or store already carries build as an int
// and cannot take the 14-digit stamp — that value does not fit in a 32-bit int.
// The day is eight digits and is safely below math.MaxInt32 on every platform
// Go supports. It is enough to know how old a build is; it cannot tell two
// builds on the same UTC day apart. Reach for Build() when the caller can
// accept a string.
func BuildDate() int {
	if len(buildNumber) < 8 {
		return 0
	}
	n, err := strconv.Atoi(buildNumber[:8])
	if err != nil {
		return 0
	}
	return n
}

// SemVer returns the release number alone — "0.1.0" — with no build metadata.
//
// Named for why you would want it rather than for its shape: this is the
// parseable form, for protocol handshakes another program may compare (MCP
// serverInfo, ACP Implementation, device handshakes) and for checking a version
// against a constraint. Reach for Version() everywhere else.
func SemVer() string { return version }

// BuildInfo returns the build timestamp and the Go toolchain version. The
// toolchain falls back to the running binary's own, which is knowable even in an
// unstamped build; the timestamp has no such fallback and stays empty.
func BuildInfo() (string, string) {
	if goVersion == "" {
		return buildTime, runtime.Version()
	}
	return buildTime, goVersion
}
