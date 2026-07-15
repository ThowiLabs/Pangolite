//go:build !linux && !windows

package app

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

type pipeTerminalProcess struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
	cmd    *exec.Cmd
	once   sync.Once
}

func (p *pipeTerminalProcess) Read(b []byte) (int, error) {
	if p == nil || p.stdout == nil {
		return 0, io.ErrClosedPipe
	}
	return p.stdout.Read(b)
}

func (p *pipeTerminalProcess) Write(b []byte) (int, error) {
	if p == nil || p.stdin == nil {
		return 0, io.ErrClosedPipe
	}
	return p.stdin.Write(b)
}

func (p *pipeTerminalProcess) Close() error {
	p.once.Do(func() {
		if p.stdin != nil {
			_ = p.stdin.Close()
		}
		if p.stdout != nil {
			_ = p.stdout.Close()
		}
		if p.cmd != nil && p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
	})
	return nil
}

func startTerminalProcess(ctx context.Context, opts terminalStartOptions) (*terminalProcess, error) {
	shell, args := fallbackTerminalShell(opts.Shell)
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Env = mergeTerminalEnv(os.Environ(), []string{"TERM=xterm-256color", "COLORTERM=truecolor"})
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	p := &pipeTerminalProcess{stdin: stdin, stdout: stdout, cmd: cmd}
	go func() {
		_ = cmd.Wait()
		_ = p.Close()
	}()
	go func() {
		<-ctx.Done()
		_ = p.Close()
	}()
	return &terminalProcess{rw: p}, nil
}

func fallbackTerminalShell(shell string) (string, []string) {
	if shell = strings.TrimSpace(shell); shell != "" {
		return shell, nil
	}
	candidates := []string{os.Getenv("SHELL"), "/bin/bash", "/bin/zsh", "/bin/ksh", "/bin/sh"}
	if runtime.GOOS == "plan9" {
		candidates = []string{os.Getenv("SHELL"), "/bin/rc"}
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, []string{"-i"}
		}
	}
	return "/bin/sh", []string{"-i"}
}
