package app

import "testing"

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
