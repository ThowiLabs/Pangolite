package app

import (
	"bytes"
	"testing"
)

type shortBufferWriter struct {
	buf bytes.Buffer
	max int
}

func (w *shortBufferWriter) Write(p []byte) (int, error) {
	if len(p) > w.max {
		p = p[:w.max]
	}
	return w.buf.Write(p)
}

func TestWriteFullCompletesShortWrites(t *testing.T) {
	w := &shortBufferWriter{max: 3}
	want := []byte("pangolite-stream")
	if err := writeFull(w, want); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.buf.Bytes(), want) {
		t.Fatalf("writeFull=%q want=%q", w.buf.Bytes(), want)
	}
}
