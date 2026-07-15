//go:build linux

package app

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInteractiveShellArgsForCommonShells(t *testing.T) {
	for _, shell := range []string{"/bin/bash", "/bin/ash", "/bin/sh", "/system/bin/sh", "/usr/bin/zsh"} {
		args := interactiveShellArgs(shell)
		if len(args) != 1 || args[0] != "-i" {
			t.Fatalf("interactiveShellArgs(%q) = %v, want [-i]", shell, args)
		}
	}
}

func TestFirstExistingShellDoesNotReturnMissingFallback(t *testing.T) {
	if got := firstExistingShell([]string{"", "/ruta/que/no/existe"}); got != "" {
		t.Fatalf("fallback shell = %q, want empty", got)
	}
}

func TestFirstExistingShellResolvesCommandFromPath(t *testing.T) {
	dir := t.TempDir()
	shellPath := filepath.Join(dir, "sh")
	if err := os.WriteFile(shellPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got := firstExistingShell([]string{"sh"}); got != shellPath {
		t.Fatalf("resolved shell = %q, want %q", got, shellPath)
	}
}

func TestDefaultShellCandidatesIncludeAndroidAndPathLookup(t *testing.T) {
	joined := strings.Join(defaultLinuxShellCandidates(), "\n")
	for _, want := range []string{"/system/bin/sh", "/system/xbin/bash", "/vendor/bin/sh", "\nsh"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("shell candidates do not include %q: %s", want, joined)
		}
	}
}

func TestResolveExecutableRejectsNonExecutableFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shell")
	if err := os.WriteFile(path, []byte("exit 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveExecutable(path); err == nil {
		t.Fatal("resolveExecutable accepted a non-executable file")
	}
}

func TestLinuxTerminalForkExecRoundTrip(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("/bin/sh is not available in the test environment")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	term, err := startTerminalProcess(ctx, terminalStartOptions{Shell: "/bin/sh", Cols: 80, Rows: 24})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	marker := "PANGOLITE_TERMINAL_FORKEXEC_OK"
	if _, err := term.Write([]byte("printf '" + marker + "\\n'; exit\n")); err != nil {
		t.Fatal(err)
	}
	result := make(chan []byte, 1)
	go func() {
		var out bytes.Buffer
		buf := make([]byte, 4096)
		for out.Len() < 64*1024 {
			n, readErr := term.Read(buf)
			if n > 0 {
				out.Write(buf[:n])
				if strings.Contains(out.String(), marker) {
					break
				}
			}
			if readErr != nil {
				break
			}
		}
		result <- out.Bytes()
	}()
	select {
	case out := <-result:
		if !bytes.Contains(out, []byte(marker)) {
			t.Fatalf("terminal output does not contain marker: %q", out)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for terminal output")
	}
}
