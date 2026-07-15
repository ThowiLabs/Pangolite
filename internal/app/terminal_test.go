package app

import (
	"bytes"
	"io"
	"strconv"
	"testing"
)

type testTerminalRW struct {
	bytes.Buffer
	closeCalls int
	maxWrite   int
}

func (rw *testTerminalRW) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (rw *testTerminalRW) Write(data []byte) (int, error) {
	if rw.maxWrite > 0 && len(data) > rw.maxWrite {
		data = data[:rw.maxWrite]
	}
	return rw.Buffer.Write(data)
}

func (rw *testTerminalRW) Close() error {
	rw.closeCalls++
	return nil
}

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

func TestDecodeTerminalControlJSONRequiresMarkerAndKnownType(t *testing.T) {
	valid := []byte(`{"pangoliteTerminal":true,"type":"resize","cols":90,"rows":28}`)
	if msg, ok := decodeTerminalControlJSON(valid); !ok || msg.Cols != 90 || msg.Rows != 28 {
		t.Fatalf("control valido rechazado: ok=%v msg=%+v", ok, msg)
	}

	for _, data := range [][]byte{
		[]byte(`{"type":"resize","cols":90,"rows":28}`),
		[]byte(`{"pangoliteTerminal":true,"type":"desconocido"}`),
		[]byte(`texto normal`),
	} {
		if msg, ok := decodeTerminalControlJSON(data); ok {
			t.Fatalf("control invalido aceptado: %q msg=%+v", data, msg)
		}
	}
}

func TestTerminalControlFilterPreservesInvalidReservedFrame(t *testing.T) {
	payload := []byte(`{"type":"resize","cols":90,"rows":28}`)
	frame := append([]byte(nil), terminalControlPrefix...)
	frame = strconv.AppendInt(frame, int64(len(payload)), 10)
	frame = append(frame, '\n')
	frame = append(frame, payload...)

	var filter terminalControlFilter
	payloads := filter.Payloads(&terminalProcess{}, frame)
	if len(payloads) != 1 || !bytes.Equal(payloads[0], frame) {
		t.Fatalf("frame invalido no preservado: %q", payloads)
	}
}

func TestWriteTerminalPayloadHandlesShortWrites(t *testing.T) {
	rw := &testTerminalRW{maxWrite: 2}
	if err := writeTerminalPayload(rw, []byte("abcdef")); err != nil {
		t.Fatalf("writeTerminalPayload fallo: %v", err)
	}
	if got := rw.String(); got != "abcdef" {
		t.Fatalf("contenido escrito = %q, want abcdef", got)
	}
}

func TestTerminalProcessCloseIsIdempotent(t *testing.T) {
	rw := &testTerminalRW{}
	term := &terminalProcess{rw: rw}
	if err := term.Close(); err != nil {
		t.Fatalf("primer cierre fallo: %v", err)
	}
	if err := term.Close(); err != nil {
		t.Fatalf("segundo cierre fallo: %v", err)
	}
	if rw.closeCalls != 1 {
		t.Fatalf("cierres del recurso = %d, want 1", rw.closeCalls)
	}
}

func TestTerminalControlFilterEmitsPlainDataBeforeSplitPrefix(t *testing.T) {
	encoded := encodeTerminalControlMessage(terminalControlMessage{Type: "resize", Cols: 120, Rows: 36})
	cut := len(terminalControlPrefix) / 2
	first := append([]byte("comando"), encoded[:cut]...)

	var gotCols, gotRows int
	term := &terminalProcess{resize: func(cols, rows int) error {
		gotCols = cols
		gotRows = rows
		return nil
	}}
	var filter terminalControlFilter
	payloads := filter.Payloads(term, first)
	if len(payloads) != 1 || !bytes.Equal(payloads[0], []byte("comando")) {
		t.Fatalf("datos normales retenidos incorrectamente: %q", payloads)
	}
	if payloads := filter.Payloads(term, encoded[cut:]); len(payloads) != 0 {
		t.Fatalf("payloads inesperados al completar control: %q", payloads)
	}
	if gotCols != 120 || gotRows != 36 {
		t.Fatalf("resize recibido = %dx%d, want 120x36", gotCols, gotRows)
	}
}

func TestMergeTerminalEnvReplacesDuplicateKeys(t *testing.T) {
	got := mergeTerminalEnv(
		[]string{"HOME=/tmp", "TERM=dumb", "KEEP=yes"},
		[]string{"HOME=/root", "TERM=xterm-256color"},
	)
	want := []string{"KEEP=yes", "HOME=/root", "TERM=xterm-256color"}
	if len(got) != len(want) {
		t.Fatalf("env = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("env[%d] = %q, want %q; env=%v", i, got[i], want[i], got)
		}
	}
}
