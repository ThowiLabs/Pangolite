//go:build !linux

package main

import "errors"

func readPasswordInteractive(string) (string, error) {
	return "", errors.New("entrada oculta no disponible en esta plataforma; usa --password-stdin")
}
