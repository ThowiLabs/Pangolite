package main

import (
	"bytes"
	"testing"

	"github.com/thowilabs/pangolite/internal/app"
)

func TestVersionCommands(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			if err := run([]string{arg}, &stdout, &stderr); err != nil {
				t.Fatalf("run(%q) devolvio error: %v", arg, err)
			}
			want := app.VersionSummary("pangolite") + "\n"
			if got := stdout.String(); got != want {
				t.Fatalf("stdout = %q, want %q", got, want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr inesperado: %q", stderr.String())
			}
		})
	}
}
