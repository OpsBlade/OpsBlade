// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package app

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestVersion_IsTheOnlySourceOfTruth pins why the const is unexported: the
// release version cannot be overwritten at link time or runtime, so no build can
// report a version its source does not declare.
func TestVersion_IsTheOnlySourceOfTruth(t *testing.T) {
	assert.NotEmpty(t, version)
	assert.NotEqual(t, "dev", version, "version must be a real release number, not a build-time placeholder")
	assert.True(t, strings.HasPrefix(Version(), version),
		"Version must lead with the release number, decorating it at most")
	assert.Equal(t, version, SemVer())
}

// TestVersion_ReleaseAndCommitAreOneToken is the reason for the "+". Rendered as
// "0.1.0 (git: abc)", a version gets pasted into a bug report as "0.1.0" — the
// space reads as the end of the value and the parenthetical as an aside, so the
// half identifying the exact source is the half that gets dropped. The release
// and the commit therefore have to survive as a single word, whatever else the
// string carries.
func TestVersion_ReleaseAndCommitAreOneToken(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "abc12345", "20260902155301"
	got := Version()

	assert.NotContains(t, got, "(", "no parentheses — they invite truncation")
	assert.Equal(t, version+"+abc12345", strings.Fields(got)[0],
		"truncating at the first space must still leave the release and the commit")
}

// TestVersion_UsesBuildMetadataSeparator pins the separator against SemVer's two
// meanings. "+" is build metadata, which the spec requires be ignored when
// comparing versions, so 0.1.0+abc compares EQUAL to 0.1.0. "-" would be a
// pre-release identifier, making the stamped build compare LOWER than the plain
// release and claim to be something that came before it.
func TestVersion_UsesBuildMetadataSeparator(t *testing.T) {
	old := gitCommit
	t.Cleanup(func() { gitCommit = old })

	gitCommit = "abc12345"
	got := Version()

	assert.Contains(t, got, "+", "the commit must attach as SemVer build metadata")
	got = strings.Fields(got)[0]
	assert.NotContains(t, strings.TrimPrefix(got, version), "-",
		"a hyphen would make this a pre-release, sorting below the release itself")
	assert.Equal(t, version, strings.SplitN(got, "+", 2)[0],
		"everything before the + must be the untouched release number")
}

// TestVersion_UnstampedBuildIsBare covers a plain `go build`: no commit, so no
// dangling separator on the end of the number.
func TestVersion_UnstampedBuildIsBare(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "", ""
	assert.Equal(t, version, Version())
	assert.NotContains(t, Version(), "+", "no dangling separator when nothing is stamped in")
	assert.NotContains(t, Version(), "[", "no empty brackets when no build number is stamped in")
	assert.Empty(t, Build())
	assert.Zero(t, BuildDate())
}

// TestSemVer_IsPlainSemver pins the protocol contract: no build metadata, ever.
// Clients may compare this field, and some of them are software you do not
// control.
func TestSemVer_IsPlainSemver(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "abc12345", "20260902155301"

	assert.Equal(t, version, SemVer())
	assert.NotContains(t, SemVer(), "+", "no build metadata on the protocol form")
	assert.NotContains(t, SemVer(), "[", "no build number on the protocol form")
	assert.NotContains(t, SemVer(), " ", "the protocol form is a single bare token")
	assert.NotEqual(t, Version(), SemVer(), "the two forms differ once a build is stamped")
}

// TestIdentityAccessors covers the remaining identity, which has no interesting
// logic but must not return empty strings into a startup banner.
func TestIdentityAccessors(t *testing.T) {
	assert.NotEmpty(t, Name())
	assert.NotEmpty(t, TagLine())
	assert.NotEmpty(t, Copyright())
}

func TestBuildInfo_UsesStampedValues(t *testing.T) {
	ob, og := buildTime, goVersion
	t.Cleanup(func() { buildTime, goVersion = ob, og })

	buildTime, goVersion = "2026-02-20T00:00:00Z", "go1.23.0"

	b, g := BuildInfo()
	assert.Equal(t, "2026-02-20T00:00:00Z", b)
	assert.Equal(t, "go1.23.0", g)
}

// TestBuildInfo_FallsBackForToolchainOnly — the toolchain is knowable from the
// running binary even unstamped; the timestamp is not, and inventing one would
// be worse than reporting none.
func TestBuildInfo_FallsBackForToolchainOnly(t *testing.T) {
	ob, og := buildTime, goVersion
	t.Cleanup(func() { buildTime, goVersion = ob, og })

	buildTime, goVersion = "", ""

	b, g := BuildInfo()
	assert.Empty(t, b)
	assert.Equal(t, runtime.Version(), g)
}

// TestVersion_CarriesTheBuildNumber is the point of the whole thing: the number
// has to be visible wherever the version is, because the surfaces that matter
// all render Version() and nothing else.
func TestVersion_CarriesTheBuildNumber(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "abc12345", "20260902155301"

	assert.Equal(t, version+"+abc12345 [20260902155301]", Version())
	assert.Equal(t, "20260902155301", Build())
	assert.Equal(t, 20260902, BuildDate())
}

// TestVersion_BuildNumberWithoutCommit covers a build stamped by something that
// sets only the number: the brackets still attach cleanly to a bare release
// rather than dangling off a stray "+".
func TestVersion_BuildNumberWithoutCommit(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "", "20260902155301"

	assert.Equal(t, version+" [20260902155301]", Version())
	assert.NotContains(t, Version(), "+", "no separator for a commit that was never stamped")
	assert.Equal(t, 20260902, BuildDate())
}

// TestBuildDate_RequiresAFullDay pins that a truncated or non-numeric stamp
// must not invent a date — 0 is the unstamped value, not a guess.
func TestBuildDate_RequiresAFullDay(t *testing.T) {
	old := buildNumber
	t.Cleanup(func() { buildNumber = old })

	buildNumber = "202609"
	assert.Zero(t, BuildDate())

	buildNumber = "YYYYMMDDHHMMSS"
	assert.Zero(t, BuildDate())
}

// TestBuildNumbersSortLexically is the property that makes the number worth
// having: string comparison has to match chronological order, because that is
// how anyone reads two of them side by side. Fixed-width zero-padded UTC
// yyyymmddhhmmss is what guarantees it — a shorter or unpadded format (say
// unix seconds, or a local-time rendering) would break at a digit boundary or
// at a DST change.
func TestBuildNumbersSortLexically(t *testing.T) {
	ordered := []string{
		"20260101000000",
		"20260902094117",
		"20260902155301",
		"20260902155302",
		"20261231235959",
		"20270101000000",
	}
	for i := 1; i < len(ordered); i++ {
		assert.Less(t, ordered[i-1], ordered[i],
			"build numbers must compare in the same order as the clock, or they cannot be read at a glance")
		assert.Len(t, ordered[i], len(ordered[0]), "every build number must be the same width")
	}
}

// TestBuildNumberShapeMatchesTheMakefile pins the format the Makefile stamps
// (date -u +%Y%m%d%H%M%S). If that format is ever changed to something narrower
// — minutes, or a local-time value — this fails rather than silently producing
// build numbers that tie or go backwards.
func TestBuildNumberShapeMatchesTheMakefile(t *testing.T) {
	stamped := time.Date(2026, 9, 2, 15, 53, 1, 0, time.UTC).Format("20060102150405")
	assert.Equal(t, "20260902155301", stamped)
	assert.Len(t, stamped, 14, "seconds resolution: two rebuilds inside a minute must differ")
}
