package app

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTerminalDownloadRejectsProtectedSystemDirectories(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("politica de rutas Linux")
	}
	for _, path := range []string{"/", "/var", "/etc", "/proc", "/var/log", "/var/spool", "/usr/local"} {
		if err := validateTerminalArchiveRoot(path); err == nil {
			t.Fatalf("%s debia estar protegido contra ZIP recursivo", path)
		}
	}
	for _, path := range []string{"/tmp/pangolite-datos", "/var/www/pangolite"} {
		if err := validateTerminalArchiveRoot(path); err != nil {
			t.Fatalf("un directorio de datos comun no debe bloquearse (%s): %v", path, err)
		}
	}
}

func TestTerminalDownloadArchiveSkipsSymlinksAndPreservesFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "uno.txt"), []byte("hola"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "dos.txt"), []byte("mundo"), 0o600); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink("/etc/passwd", filepath.Join(root, "passwd-link"))

	target, err := inspectTerminalDownloadTarget(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if target.Kind != "directory" || !strings.HasSuffix(target.Name, ".zip") {
		t.Fatalf("target inesperado: %#v", target)
	}
	var out bytes.Buffer
	if err := writeTerminalDownloadPayload(context.Background(), &out, target); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, file := range zr.File {
		seen[file.Name] = true
	}
	if !seen["uno.txt"] || !seen["sub/dos.txt"] {
		t.Fatalf("faltan archivos esperados en ZIP: %#v", seen)
	}
	if seen["passwd-link"] {
		t.Fatal("un symlink no debe entrar al ZIP")
	}
}

func TestTerminalDownloadStreamHeaderRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	want := terminalDownloadStreamMeta{Name: "reporte.zip", Kind: "directory", Size: 1234}
	if err := writeTerminalDownloadStreamHeader(&buf, want); err != nil {
		t.Fatal(err)
	}
	got, err := readTerminalDownloadStreamHeader(bufio.NewReader(&buf))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("metadata distinta: got=%#v want=%#v", got, want)
	}
}

func TestPrepareTerminalDownloadOfferUsesCurrentDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "reporte.txt")
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	term := &terminalProcess{currentDir: func() (string, error) { return root, nil }}
	msg := prepareTerminalDownloadOffer(term, terminalControlMessage{Type: "download.request", DownloadID: "down_test_123", Path: "reporte.txt"})
	if msg.Type != "download.offer" {
		t.Fatalf("se esperaba offer, got %#v", msg)
	}
	if msg.Path != path || msg.Name != "reporte.txt" || msg.Kind != "file" || msg.Size != 2 {
		t.Fatalf("offer inesperado: %#v", msg)
	}
}

func TestTerminalZipMethodAvoidsExpensiveCompressionForLargeFiles(t *testing.T) {
	if got := terminalZipMethod("logs.txt", 128<<20); got != zip.Store {
		t.Fatalf("un archivo grande debe usar Store, got %d", got)
	}
	if got := terminalZipMethod("logs.txt", 1024); got != zip.Deflate {
		t.Fatalf("un texto pequeno debe usar Deflate, got %d", got)
	}
	if got := terminalZipMethod("foto.jpg", 1024); got != zip.Store {
		t.Fatalf("un formato ya comprimido debe usar Store, got %d", got)
	}
}
