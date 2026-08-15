package app

import (
	"strconv"
	"strings"
	"testing"
)

func TestVersionSummaryUsesSharedVersionAndCode(t *testing.T) {
	originalVersion, originalCode := Version, VersionCode
	t.Cleanup(func() { Version, VersionCode = originalVersion, originalCode })
	Version = "v9.8"
	VersionCode = "900008"
	if got, want := NormalizedVersion(), "9.8"; got != want {
		t.Fatalf("NormalizedVersion() = %q, want %q", got, want)
	}
	if got, want := VersionSummary("pangolite-client"), "pangolite-client 9.8 (code 900008)"; got != want {
		t.Fatalf("VersionSummary() = %q, want %q", got, want)
	}
}

func TestVersionCodeMatchesVersion(t *testing.T) {
	version := NormalizedVersion()
	parts := strings.Split(version, ".")
	if len(parts) != 2 {
		t.Fatalf("version de desarrollo %q no usa formato X.Y", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		t.Fatalf("major invalido en %q: %v", version, err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("minor invalido en %q: %v", version, err)
	}
	want := strconv.Itoa(major*100000 + minor)
	if got := NormalizedVersionCode(); got != want {
		t.Fatalf("VersionCode = %q, want %q para version %q", got, want, version)
	}
}
