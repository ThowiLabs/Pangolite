//go:build windows

package app

import (
	"context"
	"errors"
)

var errWindowsTerminalUnstable = errors.New("la terminal remota en Windows puede fallar demasiado por diferencias entre servicios, sesiones interactivas y consola; usa Linux para consola remota estable o conectate por RDP/PowerShell/SSH mientras se implementa soporte Windows confiable")

func startTerminalProcess(ctx context.Context, opts terminalStartOptions) (*terminalProcess, error) {
	return nil, errWindowsTerminalUnstable
}
