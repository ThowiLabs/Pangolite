//go:build linux

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

func readPasswordInteractive(prompt string) (string, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return "", errors.New("no hay terminal interactiva; usa --password-stdin")
	}
	defer tty.Close()

	fd := int(tty.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return "", errors.New("no se pudo preparar la terminal; usa --password-stdin")
	}
	newState := *oldState
	newState.Lflag &^= unix.ECHO
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return "", errors.New("no se pudo ocultar la contraseña; usa --password-stdin")
	}
	defer func() { _ = unix.IoctlSetTermios(fd, unix.TCSETS, oldState) }()

	if _, err := fmt.Fprint(tty, prompt); err != nil {
		return "", err
	}
	reader := bufio.NewReader(tty)
	line, readErr := reader.ReadString('\n')
	_, _ = fmt.Fprintln(tty)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", fmt.Errorf("leer contraseña: %w", readErr)
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}
