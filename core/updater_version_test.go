package core

import (
	"strings"
	"testing"
)

// SemVer build metadata carries no precedence (SemVer 2.0 §10), but the version
// must still parse. A locally built binary is stamped `v1.5.0+toolpanel.3`, and
// treating that as 0.0.0 made every upstream release look newer — so /upgrade
// offered to "update" a local build and would overwrite it with an older tag.
func TestParseSemverIgnoresBuildMetadata(t *testing.T) {
	tests := []struct {
		in    string
		major int
		minor int
		patch int
		pre   string
	}{
		{"v1.5.0+toolpanel.3", 1, 5, 0, ""},
		{"1.5.0+build.7", 1, 5, 0, ""},
		{"v1.5.0-beta.3+toolpanel.3", 1, 5, 0, "beta.3"},
	}
	for _, tt := range tests {
		got := parseSemver(tt.in)
		if !got.valid {
			t.Fatalf("parseSemver(%q) should be valid, got %+v", tt.in, got)
		}
		if got.major != tt.major || got.minor != tt.minor || got.patch != tt.patch || got.pre != tt.pre {
			t.Fatalf("parseSemver(%q) = %+v, want %d.%d.%d pre=%q", tt.in, got, tt.major, tt.minor, tt.patch, tt.pre)
		}
	}

	// Build metadata is ignored when ordering, so the same release with and
	// without it compares equal, and a real bump still wins.
	if got := semverCompare("v1.5.0+toolpanel.3", "v1.5.0"); got != 0 {
		t.Fatalf("build metadata should not affect precedence, got %d", got)
	}
	if got := semverCompare("v1.5.1", "v1.5.0+toolpanel.3"); got <= 0 {
		t.Fatalf("v1.5.1 should outrank v1.5.0+toolpanel.3, got %d", got)
	}
	if got := semverCompare("v1.5.0", "v1.5.0-beta.3+toolpanel.3"); got <= 0 {
		t.Fatalf("release should outrank its own prerelease, got %d", got)
	}
}

// An unrecognizable version must be reported as such rather than silently
// becoming 0.0.0, which is the lowest possible version and therefore always
// "behind" every release.
func TestParseSemverMarksUnparseableInvalid(t *testing.T) {
	for _, in := range []string{"not-a-version", "dev", "", "v1.5", "1.5.0.1"} {
		if got := parseSemver(in); got.valid {
			t.Fatalf("parseSemver(%q) should be invalid, got %+v", in, got)
		}
	}
}

// CheckForUpdate must refuse to compare an unknown local version instead of
// claiming an update is available — and it must decide that before reaching the
// network, so an unknown build cannot be talked into a downgrade while offline.
func TestCheckForUpdateRejectsUnknownLocalVersion(t *testing.T) {
	release, err := CheckForUpdate("nightly-2026-08-26", true)
	if err == nil {
		t.Fatalf("expected an error for an uncomparable local version, got release %+v", release)
	}
	if release != nil {
		t.Fatalf("no release should be offered for an uncomparable local version, got %+v", release)
	}
	if !strings.Contains(err.Error(), "nightly-2026-08-26") {
		t.Fatalf("error should name the offending version, got %q", err)
	}
}

// A git-describe version (`v1.5.0-10-gabc1234`) is a valid SemVer prerelease and
// therefore orders *below* v1.5.0 — so a build made 10 commits past the tag is
// still offered v1.5.0 as an "update". That is what SemVer says, and it is
// recorded here so the behaviour is a decision rather than an accident.
func TestGitDescribeVersionOrdersBelowItsTag(t *testing.T) {
	got := parseSemver("v1.5.0-10-gabc1234")
	if !got.valid || got.pre != "10-gabc1234" {
		t.Fatalf("git-describe version should parse as a prerelease, got %+v", got)
	}
	if semverCompare("v1.5.0", "v1.5.0-10-gabc1234") <= 0 {
		t.Fatalf("SemVer orders a prerelease below its release; got %d", semverCompare("v1.5.0", "v1.5.0-10-gabc1234"))
	}
}

// A version that does parse must still reach the comparison path; this guards
// against the fix over-reaching and refusing legitimate checks.
func TestCheckForUpdateAcceptsParseableLocalVersion(t *testing.T) {
	if err := validateComparableVersion("v1.5.0+toolpanel.3"); err != nil {
		t.Fatalf("a build-stamped release should be comparable, got %v", err)
	}
	if err := validateComparableVersion("v1.5.0-beta.3"); err != nil {
		t.Fatalf("a prerelease should be comparable, got %v", err)
	}
}
