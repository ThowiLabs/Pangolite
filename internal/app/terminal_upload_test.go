package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeTerminalUploadName(t *testing.T) {
	name, err := sanitizeTerminalUploadName(`../../docs/reporte.txt`)
	if err != nil {
		t.Fatal(err)
	}
	if name != "reporte.txt" {
		t.Fatalf("nombre saneado = %q, want reporte.txt", name)
	}
	for _, invalid := range []string{".", "..", "archivo\n.txt", "archivo\x00.txt"} {
		if _, err := sanitizeTerminalUploadName(invalid); err == nil {
			t.Fatalf("sanitizeTerminalUploadName(%q) debía fallar", invalid)
		}
	}
}

func TestTerminalUploadManagerWritesToCurrentDirAndAvoidsOverwrite(t *testing.T) {
	dir := t.TempDir()
	original := filepath.Join(dir, "reporte.txt")
	if err := os.WriteFile(original, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	term := &terminalProcess{currentDir: func() (string, error) { return dir, nil }}
	var notices []terminalControlMessage
	manager := newTerminalUploadManager(term, func(msg terminalControlMessage) error {
		notices = append(notices, msg)
		return nil
	})
	defer manager.Close()

	const id = "upload_test_123"
	manager.HandleControl(terminalControlMessage{Type: "upload.start", UploadID: id, Name: "reporte.txt", Size: 6})
	if len(notices) == 0 || notices[len(notices)-1].Type != "upload.ready" {
		t.Fatalf("no se recibió upload.ready: %#v", notices)
	}
	manager.HandleChunk(id, []byte("abc"))
	manager.HandleChunk(id, []byte("def"))
	manager.HandleControl(terminalControlMessage{Type: "upload.finish", UploadID: id})

	if got, err := os.ReadFile(original); err != nil || string(got) != "original" {
		t.Fatalf("el archivo existente fue alterado: got=%q err=%v", got, err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "reporte (1).txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "abcdef" {
		t.Fatalf("contenido subido = %q, want abcdef", got)
	}
	if last := notices[len(notices)-1]; last.Type != "upload.done" || filepath.Base(last.Path) != "reporte (1).txt" {
		t.Fatalf("notificación final inesperada: %#v", last)
	}
	partials, err := filepath.Glob(filepath.Join(dir, ".pangolite-upload-*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(partials) != 0 {
		t.Fatalf("quedaron temporales después de completar: %v", partials)
	}
}

func TestTerminalUploadManagerRemovesPartialOnClose(t *testing.T) {
	dir := t.TempDir()
	term := &terminalProcess{currentDir: func() (string, error) { return dir, nil }}
	manager := newTerminalUploadManager(term, nil)
	manager.HandleControl(terminalControlMessage{Type: "upload.start", UploadID: "upload_close_123", Name: "grande.bin", Size: 1024})
	manager.HandleChunk("upload_close_123", []byte("partial"))
	manager.Close()

	partials, err := filepath.Glob(filepath.Join(dir, ".pangolite-upload-*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(partials) != 0 {
		t.Fatalf("quedaron temporales tras cerrar la sesión: %v", partials)
	}
	if _, err := os.Stat(filepath.Join(dir, "grande.bin")); !os.IsNotExist(err) {
		t.Fatalf("una subida incompleta no debe publicar el archivo final: %v", err)
	}
}

func TestTerminalUploadFrameFilterHandlesSplitFrameAndPreservesTerminalData(t *testing.T) {
	const id = "upload_frame_123"
	chunk := []byte{0, 1, 2, '\n', 255, 'x'}
	frame := append([]byte(nil), terminalUploadFramePrefix...)
	frame = append(frame, []byte(fmt.Sprintf("%s %d\n", id, len(chunk)))...)
	frame = append(frame, chunk...)
	stream := append([]byte("antes"), frame...)
	stream = append(stream, []byte("despues")...)

	var filter terminalUploadFrameFilter
	var payload bytes.Buffer
	var gotID string
	var gotChunk []byte
	cut1 := len("antes") + len(terminalUploadFramePrefix)/2
	cut2 := len("antes") + len(frame) - 2
	for _, part := range [][]byte{stream[:cut1], stream[cut1:cut2], stream[cut2:]} {
		parts, err := filter.Payloads(part, func(uploadID string, data []byte) {
			gotID = uploadID
			gotChunk = append([]byte(nil), data...)
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, plain := range parts {
			payload.Write(plain)
		}
	}
	if payload.String() != "antesdespues" {
		t.Fatalf("payload terminal = %q", payload.String())
	}
	if gotID != id || !bytes.Equal(gotChunk, chunk) {
		t.Fatalf("chunk recibido id=%q data=%v", gotID, gotChunk)
	}
}

func TestTerminalUploadFrameFilterRejectsOversizedChunk(t *testing.T) {
	const id = "upload_frame_oversized"
	frame := append([]byte(nil), terminalUploadFramePrefix...)
	frame = append(frame, []byte(fmt.Sprintf("%s %d\n", id, terminalUploadChunkMaxBytes+1))...)
	var filter terminalUploadFrameFilter
	if _, err := filter.Payloads(frame, nil); err == nil || !strings.Contains(err.Error(), "frame") {
		t.Fatalf("se esperaba error por frame sobredimensionado, got %v", err)
	}
}

func TestTerminalInputFilterDoesNotParseControlMagicInsideUploadChunk(t *testing.T) {
	const id = "upload_binary_123"
	fakeControl := encodeTerminalControlMessage(terminalControlMessage{Type: "resize", Cols: 177, Rows: 55})
	chunk := append([]byte("inicio"), fakeControl...)
	chunk = append(chunk, []byte("fin")...)
	frame := append([]byte(nil), terminalUploadFramePrefix...)
	frame = append(frame, []byte(fmt.Sprintf("%s %d\n", id, len(chunk)))...)
	frame = append(frame, chunk...)

	var filter terminalInputFilter
	var controls int
	var gotChunk []byte
	payloads, err := filter.Payloads(frame, true, func(msg terminalControlMessage) bool {
		controls++
		return true
	}, func(uploadID string, data []byte) {
		if uploadID != id {
			t.Fatalf("upload id = %q, want %q", uploadID, id)
		}
		gotChunk = append([]byte(nil), data...)
	})
	if err != nil {
		t.Fatal(err)
	}
	if controls != 0 {
		t.Fatalf("el contenido binario del archivo fue interpretado como %d controles", controls)
	}
	if len(payloads) != 0 {
		t.Fatalf("un frame de upload no debe producir entrada para el PTY: %q", bytes.Join(payloads, nil))
	}
	if !bytes.Equal(gotChunk, chunk) {
		t.Fatalf("chunk binario fue alterado")
	}

	controls = 0
	payloads, err = filter.Payloads(fakeControl, true, func(msg terminalControlMessage) bool {
		controls++
		return true
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if controls != 1 || len(payloads) != 0 {
		t.Fatalf("control fuera de upload: controls=%d payloads=%d", controls, len(payloads))
	}
}

func TestTerminalUploadManagerAllowsFilesLargerThan16MiB(t *testing.T) {
	dir := t.TempDir()
	term := &terminalProcess{currentDir: func() (string, error) { return dir, nil }}
	manager := newTerminalUploadManager(term, nil)
	defer manager.Close()

	const id = "upload_large_123"
	const size = int64(16<<20 + 12345)
	manager.HandleControl(terminalControlMessage{Type: "upload.start", UploadID: id, Name: "grande.bin", Size: size})
	chunk := make([]byte, 32<<10)
	var sent int64
	for sent < size {
		n := int64(len(chunk))
		if remaining := size - sent; remaining < n {
			n = remaining
		}
		manager.HandleChunk(id, chunk[:int(n)])
		sent += n
	}
	manager.HandleControl(terminalControlMessage{Type: "upload.finish", UploadID: id})
	info, err := os.Stat(filepath.Join(dir, "grande.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != size {
		t.Fatalf("tamaño subido = %d, want %d", info.Size(), size)
	}
}
