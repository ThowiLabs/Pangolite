//go:build linux

package app

import "testing"

func TestInteractiveShellArgsForCommonShells(t *testing.T) {
	for _, shell := range []string{"/bin/bash", "/bin/ash", "/bin/sh", "/usr/bin/zsh"} {
		args := interactiveShellArgs(shell)
		if len(args) != 1 || args[0] != "-i" {
			t.Fatalf("interactiveShellArgs(%q) = %v, want [-i]", shell, args)
		}
	}
}

func TestFirstExistingShellFallback(t *testing.T) {
	if got := firstExistingShell([]string{"", "/ruta/que/no/existe"}); got != "/bin/sh" {
		t.Fatalf("fallback shell = %q, want /bin/sh", got)
	}
}
