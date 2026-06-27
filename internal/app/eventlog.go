package app

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const defaultMaxLogLines = 1000

type RotatingLineWriter struct {
	path     string
	maxLines int
	mu       sync.Mutex
}

func NewRotatingLineWriter(path string, maxLines int) (*RotatingLineWriter, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("ruta de log requerida")
	}
	if maxLines <= 0 {
		maxLines = defaultMaxLogLines
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			return nil, err
		}
	}
	w := &RotatingLineWriter{path: path, maxLines: maxLines}
	_ = w.trimLocked()
	return w, nil
}

func (w *RotatingLineWriter) Write(p []byte) (int, error) {
	if w == nil {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	_, writeErr := f.Write(p)
	closeErr := f.Close()
	if writeErr != nil {
		return 0, writeErr
	}
	if closeErr != nil {
		return 0, closeErr
	}
	if err := w.trimLocked(); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *RotatingLineWriter) trimLocked() error {
	data, err := os.ReadFile(w.path)
	if err != nil {
		return err
	}
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= w.maxLines {
		return nil
	}
	lines = lines[len(lines)-w.maxLines:]
	trimmed := bytes.Join(lines, []byte("\n"))
	trimmed = append(trimmed, '\n')
	return os.WriteFile(w.path, trimmed, 0o600)
}

func ReadLastLogLines(path string, maxLines int) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("ruta de log no configurada")
	}
	if maxLines <= 0 || maxLines > defaultMaxLogLines {
		maxLines = defaultMaxLogLines
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(parts) == 1 && parts[0] == "" {
		return []string{}, nil
	}
	if len(parts) > maxLines {
		parts = parts[len(parts)-maxLines:]
	}
	return parts, nil
}

func NewMultiLogWriter(stdout io.Writer, path string) (io.Writer, error) {
	fileWriter, err := NewRotatingLineWriter(path, defaultMaxLogLines)
	if err != nil {
		return stdout, err
	}
	return io.MultiWriter(stdout, fileWriter), nil
}
