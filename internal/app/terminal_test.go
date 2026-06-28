package app

import (
	"bytes"
	"testing"
)

func TestTerminalControlFilterConsumesEncodedResize(t *testing.T) {
	var gotCols, gotRows int
	term := &terminalProcess{
		resize: func(cols, rows int) error {
			gotCols = cols
			gotRows = rows
			return nil
		},
	}
	encoded := encodeTerminalControlMessage(terminalControlMessage{Type: "resize", Cols: 132, Rows: 40})
	data := append([]byte("abc"), encoded...)
	data = append(data, []byte("def")...)

	var filter terminalControlFilter
	payloads := filter.Payloads(term, data)

	if gotCols != 132 || gotRows != 40 {
		t.Fatalf("resize recibido = %dx%d, want 132x40", gotCols, gotRows)
	}
	if len(payloads) != 2 {
		t.Fatalf("payloads = %d, want 2", len(payloads))
	}
	if !bytes.Equal(payloads[0], []byte("abc")) || !bytes.Equal(payloads[1], []byte("def")) {
		t.Fatalf("payloads = %q, want abc/def", payloads)
	}
}

func TestTerminalControlFilterHandlesSplitFrame(t *testing.T) {
	var gotCols, gotRows int
	term := &terminalProcess{
		resize: func(cols, rows int) error {
			gotCols = cols
			gotRows = rows
			return nil
		},
	}
	encoded := encodeTerminalControlMessage(terminalControlMessage{Type: "resize", Cols: 100, Rows: 30})
	cut := len(terminalControlPrefix) / 2

	var filter terminalControlFilter
	if payloads := filter.Payloads(term, encoded[:cut]); len(payloads) != 0 {
		t.Fatalf("payloads parciales = %d, want 0", len(payloads))
	}
	if gotCols != 0 || gotRows != 0 {
		t.Fatalf("resize aplicado antes de completar frame = %dx%d", gotCols, gotRows)
	}
	if payloads := filter.Payloads(term, encoded[cut:]); len(payloads) != 0 {
		t.Fatalf("payloads finales = %d, want 0", len(payloads))
	}
	if gotCols != 100 || gotRows != 30 {
		t.Fatalf("resize recibido = %dx%d, want 100x30", gotCols, gotRows)
	}
}

func TestExpandStandaloneBackspaceForVT(t *testing.T) {
	got := expandStandaloneBackspaceForVT([]byte("abc\b"))
	want := []byte("abc\b \b")
	if !bytes.Equal(got, want) {
		t.Fatalf("salida = %q, want %q", got, want)
	}
}

func TestExpandStandaloneBackspaceForVTKeepsDestructiveSequence(t *testing.T) {
	got := expandStandaloneBackspaceForVT([]byte("abc\b \b"))
	want := []byte("abc\b \b")
	if !bytes.Equal(got, want) {
		t.Fatalf("salida = %q, want %q", got, want)
	}
}
