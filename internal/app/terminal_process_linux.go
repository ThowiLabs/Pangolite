//go:build linux

package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	linuxTIOCGPTN   = 0x80045430
	linuxTIOCSPTLCK = 0x40045431
)

type terminalWinsize struct {
	Rows uint16
	Cols uint16
	X    uint16
	Y    uint16
}

type linuxTerminalCommand struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}

func startTerminalProcess(ctx context.Context, opts terminalStartOptions) (*terminalProcess, error) {
	master, slave, err := openLinuxPTY()
	if err != nil {
		return nil, err
	}
	cols, rows := normalizeTerminalSize(opts.Cols, opts.Rows)
	_ = resizePTY(master, cols, rows)

	termCmd := linuxShellCommand(ctx, opts.Shell)
	cmd := exec.CommandContext(ctx, termCmd.Path, termCmd.Args...)
	cmd.Env = append(os.Environ(), termCmd.Env...)
	cmd.Dir = termCmd.Dir
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		_ = slave.Close()
		_ = master.Close()
		return nil, err
	}
	_ = slave.Close()
	go func() {
		_ = cmd.Wait()
		_ = master.Close()
	}()
	return &terminalProcess{
		rw: master,
		resize: func(cols, rows int) error {
			cols, rows = normalizeTerminalSize(cols, rows)
			return resizePTY(master, cols, rows)
		},
	}, nil
}

func openLinuxPTY() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	unlock := int32(0)
	if err := ioctl(master.Fd(), linuxTIOCSPTLCK, uintptr(unsafe.Pointer(&unlock))); err != nil {
		_ = master.Close()
		return nil, nil, err
	}
	var n uint32
	if err := ioctl(master.Fd(), linuxTIOCGPTN, uintptr(unsafe.Pointer(&n))); err != nil {
		_ = master.Close()
		return nil, nil, err
	}
	slaveName := "/dev/pts/" + strconv.Itoa(int(n))
	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		return nil, nil, err
	}
	return master, slave, nil
}

func resizePTY(file *os.File, cols, rows int) error {
	if file == nil {
		return nil
	}
	ws := terminalWinsize{Rows: uint16(rows), Cols: uint16(cols)}
	return ioctl(file.Fd(), uintptr(syscall.TIOCSWINSZ), uintptr(unsafe.Pointer(&ws)))
}

func ioctl(fd uintptr, req uintptr, arg uintptr) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, req, arg)
	if errno != 0 {
		return errno
	}
	return nil
}

func linuxShellCommand(ctx context.Context, shell string) linuxTerminalCommand {
	if shell = strings.TrimSpace(shell); shell != "" {
		return linuxTerminalCommand{Path: shell, Args: interactiveShellArgs(shell), Dir: terminalWorkingDir(), Env: terminalIdentityEnv()}
	}
	if os.Geteuid() != 0 && canUsePasswordlessSudo(ctx) {
		return linuxTerminalCommand{Path: "sudo", Args: []string{"-n", "-i"}, Dir: "/", Env: []string{"TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}}
	}
	shell = firstExistingShell([]string{os.Getenv("SHELL"), "/bin/bash", "/bin/ash", "/bin/sh"})
	return linuxTerminalCommand{Path: shell, Args: interactiveShellArgs(shell), Dir: terminalWorkingDir(), Env: terminalIdentityEnv()}
}

func firstExistingShell(candidates []string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return "/bin/sh"
}

func interactiveShellArgs(shell string) []string {
	base := filepath.Base(shell)
	switch base {
	case "bash", "ash", "dash", "sh", "zsh", "ksh":
		return []string{"-i"}
	default:
		return nil
	}
}

func terminalWorkingDir() string {
	if os.Geteuid() == 0 {
		if isUsableDir("/root") {
			return "/root"
		}
		return "/"
	}
	if home, err := os.UserHomeDir(); err == nil && isUsableDir(home) {
		return home
	}
	return "/"
}

func terminalIdentityEnv() []string {
	env := []string{"TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	if os.Geteuid() == 0 {
		home := "/root"
		if !isUsableDir(home) {
			home = "/"
		}
		return append(env, "HOME="+home, "USER=root", "LOGNAME=root")
	}
	return env
}

func isUsableDir(path string) bool {
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		return false
	}
	dir, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = dir.Close()
	return true
}

func canUsePasswordlessSudo(ctx context.Context) bool {
	if _, err := exec.LookPath("sudo"); err != nil {
		return false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, "sudo", "-n", "true")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func normalizeTerminalSize(cols, rows int) (int, int) {
	if cols < 20 || cols > 400 {
		cols = 80
	}
	if rows < 5 || rows > 120 {
		rows = 24
	}
	return cols, rows
}
