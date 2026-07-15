//go:build linux

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

type linuxChildProcess struct {
	mu     sync.Mutex
	pid    int
	exited bool
}

func startTerminalProcess(ctx context.Context, opts terminalStartOptions) (*terminalProcess, error) {
	master, slave, err := openLinuxPTY()
	if err != nil {
		return nil, fmt.Errorf("abrir PTY: %w", err)
	}
	cols, rows := normalizeTerminalSize(opts.Cols, opts.Rows)
	_ = resizePTY(master, cols, rows)

	termCmd, err := linuxShellCommand(ctx, opts.Shell)
	if err != nil {
		_ = slave.Close()
		_ = master.Close()
		return nil, err
	}
	child, err := forkLinuxTerminal(termCmd, slave)
	_ = slave.Close()
	if err != nil {
		_ = master.Close()
		return nil, fmt.Errorf("iniciar shell %s: %w", termCmd.Path, err)
	}

	term := &terminalProcess{
		rw: master,
		resize: func(cols, rows int) error {
			cols, rows = normalizeTerminalSize(cols, rows)
			return resizePTY(master, cols, rows)
		},
		stop: child.stop,
	}
	done := make(chan struct{})
	go func() {
		child.wait()
		_ = master.Close()
		close(done)
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = term.Close()
		case <-done:
		}
	}()
	return term, nil
}

// forkLinuxTerminal usa syscall.ForkExec directamente para evitar la comprobación
// pidfd de os.StartProcess. Algunos Android y kernels antiguos bloquean pidfd_open
// con SIGSYS en lugar de devolver ENOSYS, lo que cerraría todo pangolite-client.
func forkLinuxTerminal(termCmd linuxTerminalCommand, slave *os.File) (*linuxChildProcess, error) {
	if slave == nil {
		return nil, errors.New("PTY esclava no disponible")
	}
	argv := make([]string, 0, len(termCmd.Args)+1)
	argv = append(argv, termCmd.Path)
	argv = append(argv, termCmd.Args...)
	attr := &syscall.ProcAttr{
		Dir:   termCmd.Dir,
		Env:   mergeTerminalEnv(os.Environ(), termCmd.Env),
		Files: []uintptr{slave.Fd(), slave.Fd(), slave.Fd()},
		Sys: &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
			Ctty:    0,
		},
	}
	pid, err := syscall.ForkExec(termCmd.Path, argv, attr)
	if err != nil {
		return nil, err
	}
	return &linuxChildProcess{pid: pid}, nil
}

func (p *linuxChildProcess) stop() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.exited || p.pid <= 0 {
		return nil
	}
	// El shell es líder de una sesión nueva, así que el PID también es su PGID.
	// Matar el grupo evita dejar procesos iniciados desde la terminal en segundo plano.
	if err := syscall.Kill(-p.pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		if fallbackErr := syscall.Kill(p.pid, syscall.SIGKILL); fallbackErr != nil && fallbackErr != syscall.ESRCH {
			return fallbackErr
		}
	}
	return nil
}

func (p *linuxChildProcess) wait() {
	if p == nil || p.pid <= 0 {
		return
	}
	for {
		_, err := syscall.Wait4(p.pid, nil, 0, nil)
		if err == syscall.EINTR {
			continue
		}
		break
	}
	p.mu.Lock()
	p.exited = true
	p.mu.Unlock()
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

func linuxShellCommand(ctx context.Context, shell string) (linuxTerminalCommand, error) {
	if shell = strings.TrimSpace(shell); shell != "" {
		resolved, err := resolveExecutable(shell)
		if err != nil {
			return linuxTerminalCommand{}, fmt.Errorf("shell solicitada %q no disponible: %w", shell, err)
		}
		return linuxTerminalCommand{Path: resolved, Args: interactiveShellArgs(resolved), Dir: terminalWorkingDir(), Env: terminalIdentityEnv(resolved)}, nil
	}
	if os.Geteuid() != 0 {
		if sudo, ok := passwordlessSudoPath(ctx); ok {
			return linuxTerminalCommand{Path: sudo, Args: []string{"-n", "-i"}, Dir: "/", Env: []string{"TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}}, nil
		}
	}
	resolved := firstExistingShell(defaultLinuxShellCandidates())
	if resolved == "" {
		return linuxTerminalCommand{}, errors.New("no se encontró una shell ejecutable; se buscó SHELL, shells Linux y sh mediante PATH")
	}
	return linuxTerminalCommand{Path: resolved, Args: interactiveShellArgs(resolved), Dir: terminalWorkingDir(), Env: terminalIdentityEnv(resolved)}, nil
}

func defaultLinuxShellCandidates() []string {
	return []string{
		os.Getenv("SHELL"),
		"/bin/bash", "/usr/bin/bash",
		"/bin/zsh", "/usr/bin/zsh",
		"/bin/ash", "/usr/bin/ash",
		"/bin/dash", "/usr/bin/dash",
		"/bin/ksh", "/usr/bin/ksh",
		"/bin/sh", "/usr/bin/sh",
		"/system/bin/sh",
		"/system/xbin/bash",
		"/vendor/bin/sh",
		"sh",
	}
}

func firstExistingShell(candidates []string) string {
	for _, candidate := range candidates {
		resolved, err := resolveExecutable(candidate)
		if err == nil {
			return resolved
		}
	}
	return ""
}

func resolveExecutable(candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", errors.New("ruta vacía")
	}
	if !strings.ContainsRune(candidate, filepath.Separator) {
		return exec.LookPath(candidate)
	}
	st, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("%s es un directorio", candidate)
	}
	if st.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("%s no tiene permiso de ejecución", candidate)
	}
	return candidate, nil
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
		if home := strings.TrimSpace(os.Getenv("HOME")); isUsableDir(home) {
			return home
		}
		return "/"
	}
	if home, err := os.UserHomeDir(); err == nil && isUsableDir(home) {
		return home
	}
	return "/"
}

func terminalIdentityEnv(shell string) []string {
	env := []string{"TERM=xterm-256color", "COLORTERM=truecolor", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "SHELL=" + shell}
	if os.Geteuid() == 0 {
		home := "/root"
		if !isUsableDir(home) {
			if inherited := strings.TrimSpace(os.Getenv("HOME")); isUsableDir(inherited) {
				home = inherited
			} else {
				home = "/"
			}
		}
		return append(env, "HOME="+home, "USER=root", "LOGNAME=root")
	}
	return env
}

func isUsableDir(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
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

func passwordlessSudoPath(ctx context.Context) (string, bool) {
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return "", false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := runLinuxCommandCompat(checkCtx, sudo, []string{sudo, "-n", "true"}); err != nil {
		return "", false
	}
	return sudo, true
}

func runLinuxCommandCompat(ctx context.Context, path string, argv []string) error {
	devNull, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()
	pid, err := syscall.ForkExec(path, argv, &syscall.ProcAttr{
		Env:   os.Environ(),
		Files: []uintptr{devNull.Fd(), devNull.Fd(), devNull.Fd()},
	})
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		var status syscall.WaitStatus
		for {
			_, waitErr := syscall.Wait4(pid, &status, 0, nil)
			if waitErr == syscall.EINTR {
				continue
			}
			if waitErr != nil {
				done <- waitErr
				return
			}
			if !status.Exited() || status.ExitStatus() != 0 {
				done <- fmt.Errorf("proceso terminó con estado %d", status.ExitStatus())
				return
			}
			done <- nil
			return
		}
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = syscall.Kill(pid, syscall.SIGKILL)
		<-done
		return ctx.Err()
	}
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
