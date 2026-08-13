package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

const (
	terminalUploadChunkMaxBytes = 64 << 10
	terminalUploadProgressStep  = 512 << 10
)

var terminalUploadFramePrefix = []byte("\x00PANGOLITE-TERMINAL-UPLOAD ")

type terminalUpload struct {
	id            string
	name          string
	requestedName string
	size          int64
	written       int64
	lastProgress  int64
	tempPath      string
	finalPath     string
	file          *os.File
}

type terminalUploadManager struct {
	term   *terminalProcess
	active map[string]*terminalUpload
	notify func(terminalControlMessage) error
}

func newTerminalUploadManager(term *terminalProcess, notify func(terminalControlMessage) error) *terminalUploadManager {
	return &terminalUploadManager{term: term, active: map[string]*terminalUpload{}, notify: notify}
}

func (m *terminalUploadManager) HandleControl(msg terminalControlMessage) {
	if m == nil {
		return
	}
	switch msg.Type {
	case "upload.start":
		m.start(msg)
	case "upload.finish":
		m.finish(msg.UploadID)
	case "upload.cancel":
		m.abort(msg.UploadID, "subida cancelada")
	case "resize":
		applyTerminalControl(m.term, msg)
	}
}

func (m *terminalUploadManager) HandleChunk(uploadID string, data []byte) {
	if m == nil || uploadID == "" {
		return
	}
	upload := m.active[uploadID]
	if upload == nil {
		m.sendError(uploadID, "subida no iniciada")
		return
	}
	if int64(len(data))+upload.written > upload.size {
		m.abort(uploadID, "la subida recibio mas bytes de los declarados")
		return
	}
	if err := writeTerminalPayload(upload.file, data); err != nil {
		m.abort(uploadID, "no se pudo escribir el archivo: "+err.Error())
		return
	}
	upload.written += int64(len(data))
	if upload.written-upload.lastProgress >= terminalUploadProgressStep || upload.written == upload.size {
		upload.lastProgress = upload.written
		m.send(terminalControlMessage{Type: "upload.progress", UploadID: upload.id, Name: upload.name, Size: upload.size, Written: upload.written, Path: upload.finalPath})
	}
}

func (m *terminalUploadManager) Close() {
	if m == nil {
		return
	}
	for id := range m.active {
		m.abort(id, "")
	}
}

func (m *terminalUploadManager) start(msg terminalControlMessage) {
	if !validTerminalUploadID(msg.UploadID) {
		m.sendError(msg.UploadID, "identificador de subida invalido")
		return
	}
	if _, exists := m.active[msg.UploadID]; exists {
		m.sendError(msg.UploadID, "la subida ya esta activa")
		return
	}
	if msg.Size < 0 {
		m.sendError(msg.UploadID, "tamaño de archivo invalido")
		return
	}
	name, err := sanitizeTerminalUploadName(msg.Name)
	if err != nil {
		m.sendError(msg.UploadID, err.Error())
		return
	}
	cwd, err := m.term.CurrentDir()
	if err != nil {
		m.sendError(msg.UploadID, "no se pudo resolver el directorio actual: "+err.Error())
		return
	}
	finalPath, err := availableTerminalUploadPath(cwd, name)
	if err != nil {
		m.sendError(msg.UploadID, err.Error())
		return
	}
	file, err := os.CreateTemp(cwd, ".pangolite-upload-*.part")
	if err != nil {
		m.sendError(msg.UploadID, "no se pudo crear el archivo temporal: "+err.Error())
		return
	}
	upload := &terminalUpload{id: msg.UploadID, name: filepath.Base(finalPath), requestedName: name, size: msg.Size, tempPath: file.Name(), finalPath: finalPath, file: file}
	m.active[msg.UploadID] = upload
	m.send(terminalControlMessage{Type: "upload.ready", UploadID: upload.id, Name: upload.name, Size: upload.size, Written: 0, Path: upload.finalPath})
}

func (m *terminalUploadManager) finish(uploadID string) {
	upload := m.active[uploadID]
	if upload == nil {
		m.sendError(uploadID, "subida no iniciada")
		return
	}
	if upload.written != upload.size {
		m.abort(uploadID, fmt.Sprintf("subida incompleta: %d de %d bytes", upload.written, upload.size))
		return
	}
	if err := upload.file.Close(); err != nil {
		upload.file = nil
		m.abort(uploadID, "no se pudo cerrar el archivo temporal: "+err.Error())
		return
	}
	upload.file = nil
	finalPath, err := linkTerminalUploadWithoutOverwrite(upload.tempPath, filepath.Dir(upload.finalPath), upload.requestedName)
	if err != nil {
		m.abort(uploadID, "no se pudo completar la subida: "+err.Error())
		return
	}
	upload.finalPath = finalPath
	upload.name = filepath.Base(finalPath)
	if err := os.Remove(upload.tempPath); err != nil {
		_ = os.Remove(upload.finalPath)
		m.abort(uploadID, "la subida termino pero no se pudo limpiar el temporal: "+err.Error())
		return
	}
	upload.tempPath = ""
	delete(m.active, uploadID)
	m.send(terminalControlMessage{Type: "upload.done", UploadID: upload.id, Name: upload.name, Size: upload.size, Written: upload.written, Path: upload.finalPath})
}

func (m *terminalUploadManager) abort(uploadID, reason string) {
	upload := m.active[uploadID]
	if upload != nil {
		delete(m.active, uploadID)
		if upload.file != nil {
			_ = upload.file.Close()
		}
		if upload.tempPath != "" {
			_ = os.Remove(upload.tempPath)
		}
	}
	if reason != "" {
		m.sendError(uploadID, reason)
	}
}

func (m *terminalUploadManager) sendError(uploadID, message string) {
	m.send(terminalControlMessage{Type: "upload.error", UploadID: uploadID, Error: message})
}

func (m *terminalUploadManager) send(msg terminalControlMessage) {
	if m == nil || m.notify == nil {
		return
	}
	_ = m.notify(msg)
}

func sanitizeTerminalUploadName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", errors.New("nombre de archivo invalido")
	}
	if len([]byte(name)) > 240 {
		return "", errors.New("nombre de archivo demasiado largo")
	}
	for _, r := range name {
		if r == 0 || unicode.IsControl(r) {
			return "", errors.New("nombre de archivo contiene caracteres no permitidos")
		}
	}
	return name, nil
}

func availableTerminalUploadPath(dir, name string) (string, error) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if !filepath.IsAbs(dir) {
		return "", errors.New("directorio actual invalido")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return "", errors.New("el directorio actual ya no esta disponible")
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 0; i <= 9999; i++ {
		candidateName := name
		if i > 0 {
			candidateName = fmt.Sprintf("%s (%d)%s", base, i, ext)
		}
		candidate := filepath.Join(dir, candidateName)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("comprobar destino de subida: %w", err)
		}
	}
	return "", errors.New("no se encontro un nombre disponible para la subida")
}

func linkTerminalUploadWithoutOverwrite(tempPath, dir, name string) (string, error) {
	for attempts := 0; attempts < 32; attempts++ {
		finalPath, err := availableTerminalUploadPath(dir, name)
		if err != nil {
			return "", err
		}
		if err := os.Link(tempPath, finalPath); err == nil {
			return finalPath, nil
		} else if errors.Is(err, os.ErrExist) {
			continue
		} else {
			return "", err
		}
	}
	return "", errors.New("no se encontro un nombre disponible para completar la subida")
}

func validTerminalUploadID(id string) bool {
	if len(id) < 8 || len(id) > 96 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

type terminalInputFilter struct {
	control terminalControlFilter
	upload  terminalUploadFrameFilter
}

func (f *terminalInputFilter) Payloads(data []byte, framedControl bool, onControl func(terminalControlMessage) bool, onChunk func(string, []byte)) ([][]byte, error) {
	uploadPayloads, err := f.upload.Payloads(data, onChunk)
	if err != nil {
		return nil, err
	}
	if !framedControl {
		return uploadPayloads, nil
	}
	out := make([][]byte, 0, len(uploadPayloads))
	for _, payload := range uploadPayloads {
		out = append(out, f.control.Process(payload, onControl)...)
	}
	return out, nil
}

type terminalUploadFrameFilter struct {
	buf []byte
}

func (f *terminalUploadFrameFilter) Payloads(data []byte, onChunk func(string, []byte)) ([][]byte, error) {
	if len(f.buf) > 0 {
		data = append(append([]byte(nil), f.buf...), data...)
		f.buf = nil
	}
	var payloads [][]byte
	for len(data) > 0 {
		idx := bytes.Index(data, terminalUploadFramePrefix)
		if idx < 0 {
			if fragmentLen := terminalPrefixFragmentLen(data, terminalUploadFramePrefix); fragmentLen > 0 {
				plainLen := len(data) - fragmentLen
				if plainLen > 0 {
					payloads = append(payloads, append([]byte(nil), data[:plainLen]...))
				}
				f.buf = append(f.buf[:0], data[plainLen:]...)
				return payloads, nil
			}
			payloads = append(payloads, append([]byte(nil), data...))
			return payloads, nil
		}
		if idx > 0 {
			payloads = append(payloads, append([]byte(nil), data[:idx]...))
			data = data[idx:]
		}
		afterPrefix := data[len(terminalUploadFramePrefix):]
		nl := bytes.IndexByte(afterPrefix, '\n')
		if nl < 0 {
			if len(data) > len(terminalUploadFramePrefix)+160 {
				payloads = append(payloads, append([]byte(nil), data[:1]...))
				data = data[1:]
				continue
			}
			f.buf = append(f.buf[:0], data...)
			return payloads, nil
		}
		header := strings.Fields(string(afterPrefix[:nl]))
		if len(header) != 2 || !validTerminalUploadID(header[0]) {
			payloads = append(payloads, append([]byte(nil), data[:1]...))
			data = data[1:]
			continue
		}
		chunkLen, err := strconv.Atoi(header[1])
		if err != nil || chunkLen < 0 || chunkLen > terminalUploadChunkMaxBytes {
			return payloads, errors.New("frame de subida invalido")
		}
		payloadStart := len(terminalUploadFramePrefix) + nl + 1
		frameEnd := payloadStart + chunkLen
		if len(data) < frameEnd {
			f.buf = append(f.buf[:0], data...)
			return payloads, nil
		}
		if onChunk != nil {
			onChunk(header[0], data[payloadStart:frameEnd])
		}
		data = data[frameEnd:]
	}
	return payloads, nil
}

func terminalPrefixFragmentLen(data, prefix []byte) int {
	max := len(data)
	if max > len(prefix)-1 {
		max = len(prefix) - 1
	}
	for n := max; n > 0; n-- {
		if bytes.Equal(data[len(data)-n:], prefix[:n]) {
			return n
		}
	}
	return 0
}

func jsonTerminalControlMessage(msg terminalControlMessage) ([]byte, error) {
	msg.PangoliteTerminal = true
	return json.Marshal(msg)
}

func encodeTerminalControlPayload(payload []byte) []byte {
	out := make([]byte, 0, len(terminalControlPrefix)+16+len(payload))
	out = append(out, terminalControlPrefix...)
	out = strconv.AppendInt(out, int64(len(payload)), 10)
	out = append(out, '\n')
	out = append(out, payload...)
	return out
}
