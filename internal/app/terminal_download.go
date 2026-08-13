package app

import (
	"archive/zip"
	"bufio"
	"compress/flate"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

const (
	terminalDownloadTicketTTL        = 2 * time.Minute
	terminalDownloadMaxTickets       = 64
	terminalArchiveMaxEntries        = 20000
	terminalArchiveMaxBytes    int64 = 8 << 30
	terminalDownloadHeaderMax        = 8 << 10
	terminalDownloadCopyBuffer       = 64 << 10
)

var (
	terminalDownloadStreamPrefix = []byte("PANGOLITE-DOWNLOAD/1 ")
	terminalArchiveSlots         = make(chan struct{}, 1)
)

type terminalDownloadTarget struct {
	Path    string
	Name    string
	Kind    string
	Size    int64
	Entries int
}

type terminalDownloadTicket struct {
	Token     string
	UserID    int64
	Target    string
	AgentID   string
	Path      string
	Name      string
	Kind      string
	Size      int64
	ExpiresAt time.Time
}

type terminalDownloadStreamMeta struct {
	Name  string `json:"name,omitempty"`
	Kind  string `json:"kind,omitempty"`
	Size  int64  `json:"size,omitempty"`
	Error string `json:"error,omitempty"`
}

func prepareTerminalDownloadOffer(term *terminalProcess, msg terminalControlMessage) terminalControlMessage {
	offer := terminalControlMessage{Type: "download.error", DownloadID: msg.DownloadID}
	if !validTerminalTransferID(msg.DownloadID) {
		offer.Error = "identificador de descarga invalido"
		return offer
	}
	rawPath := strings.TrimSpace(msg.Path)
	if rawPath == "" {
		offer.Error = "indica el archivo o directorio a descargar"
		return offer
	}
	if len([]byte(rawPath)) > 4096 || strings.IndexByte(rawPath, 0) >= 0 {
		offer.Error = "ruta de descarga invalida"
		return offer
	}
	cwd, err := term.CurrentDir()
	if err != nil {
		offer.Error = "no se pudo resolver el directorio actual: " + err.Error()
		return offer
	}
	path := rawPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	target, err := inspectTerminalDownloadTarget(path, false)
	if err != nil {
		offer.Error = err.Error()
		return offer
	}
	return terminalControlMessage{
		Type:       "download.offer",
		DownloadID: msg.DownloadID,
		Path:       target.Path,
		Name:       target.Name,
		Kind:       target.Kind,
		Size:       target.Size,
	}
}

func inspectTerminalDownloadTarget(path string, preflightArchive bool) (terminalDownloadTarget, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return terminalDownloadTarget{}, errors.New("ruta de descarga vacia")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return terminalDownloadTarget{}, fmt.Errorf("resolver ruta de descarga: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return terminalDownloadTarget{}, fmt.Errorf("no se encontro la ruta solicitada: %w", err)
	}
	resolved = filepath.Clean(resolved)
	info, err := os.Stat(resolved)
	if err != nil {
		return terminalDownloadTarget{}, fmt.Errorf("consultar descarga: %w", err)
	}
	if info.Mode().IsRegular() {
		return terminalDownloadTarget{Path: resolved, Name: safeTerminalDownloadName(info.Name(), "archivo.bin"), Kind: "file", Size: info.Size()}, nil
	}
	if !info.IsDir() {
		return terminalDownloadTarget{}, errors.New("solo se pueden descargar archivos regulares o directorios")
	}
	if err := validateTerminalArchiveRoot(resolved); err != nil {
		return terminalDownloadTarget{}, err
	}
	target := terminalDownloadTarget{Path: resolved, Name: safeTerminalDownloadName(info.Name()+".zip", "directorio.zip"), Kind: "directory"}
	if preflightArchive {
		entries, size, err := preflightTerminalArchive(resolved)
		if err != nil {
			return terminalDownloadTarget{}, err
		}
		target.Entries = entries
		target.Size = size
	}
	return target, nil
}

func validateTerminalArchiveRoot(path string) error {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return errors.New("el directorio de descarga debe ser absoluto")
	}
	if runtime.GOOS != "windows" && path == "/var" {
		return errors.New("Pangolite no comprime /var completo; entra a un directorio de datos concreto como /var/www")
	}
	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(path)
		if volume != "" && strings.EqualFold(path, volume+string(filepath.Separator)) {
			return errors.New("Pangolite no comprime la raiz completa de una unidad del sistema")
		}
	}
	for _, protected := range protectedTerminalArchiveRoots() {
		if pathWithin(path, protected) {
			return fmt.Errorf("Pangolite no comprime directorios de sistema (%s); entra a un directorio de datos o descarga un archivo concreto", protected)
		}
	}
	return nil
}

func protectedTerminalArchiveRoots() []string {
	if runtime.GOOS == "windows" {
		roots := []string{}
		if windir := strings.TrimSpace(os.Getenv("WINDIR")); windir != "" {
			roots = append(roots, filepath.Clean(windir))
		}
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramData"} {
			if value := strings.TrimSpace(os.Getenv(env)); value != "" {
				roots = append(roots, filepath.Clean(value))
			}
		}
		return roots
	}
	return []string{"/", "/proc", "/sys", "/dev", "/run", "/boot", "/etc", "/usr", "/bin", "/sbin", "/lib", "/lib64", "/var/lib", "/var/cache", "/var/log", "/var/spool", "/var/backups", "/var/lock", "/snap", "/lost+found"}
}

func pathWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
		root = strings.ToLower(root)
	}
	if path == root {
		return true
	}
	return strings.HasPrefix(path, root+string(filepath.Separator))
}

func preflightTerminalArchive(root string) (int, int64, error) {
	if err := validateTerminalArchiveRoot(root); err != nil {
		return 0, 0, err
	}
	entries := 0
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		entries++
		if entries > terminalArchiveMaxEntries {
			return fmt.Errorf("el directorio supera el limite seguro de %d entradas", terminalArchiveMaxEntries)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() < 0 || total > terminalArchiveMaxBytes-info.Size() {
			return fmt.Errorf("el directorio supera el limite seguro de %s sin comprimir", formatTerminalDownloadSize(terminalArchiveMaxBytes))
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return 0, 0, fmt.Errorf("no se puede preparar el ZIP: %w", err)
	}
	return entries, total, nil
}

func writeTerminalDownloadPayload(ctx context.Context, w io.Writer, target terminalDownloadTarget) error {
	if target.Kind == "file" {
		file, err := os.Open(target.Path)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.CopyBuffer(&contextWriter{ctx: ctx, w: w}, &io.LimitedReader{R: file, N: target.Size}, make([]byte, terminalDownloadCopyBuffer))
		return err
	}
	if target.Kind != "directory" {
		return errors.New("tipo de descarga no soportado")
	}
	return writeTerminalArchive(ctx, w, target.Path)
}

func writeTerminalArchive(ctx context.Context, w io.Writer, root string) error {
	select {
	case terminalArchiveSlots <- struct{}{}:
		defer func() { <-terminalArchiveSlots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	zw := zip.NewWriter(&contextWriter{ctx: ctx, w: w})
	zw.RegisterCompressor(zip.Deflate, func(dst io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(dst, flate.BestSpeed)
	})
	entries := 0
	var total int64
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if path == root {
			return nil
		}
		entries++
		if entries > terminalArchiveMaxEntries {
			return fmt.Errorf("el directorio cambio y ahora supera %d entradas", terminalArchiveMaxEntries)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return errors.New("ruta invalida dentro del directorio")
		}
		name := filepath.ToSlash(rel)
		if entry.IsDir() {
			header := &zip.FileHeader{Name: strings.TrimSuffix(name, "/") + "/", Method: zip.Store}
			header.SetMode(0o755 | os.ModeDir)
			_, err := zw.CreateHeader(header)
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if info.Size() < 0 || total > terminalArchiveMaxBytes-info.Size() {
			return fmt.Errorf("el directorio cambio y ahora supera %s", formatTerminalDownloadSize(terminalArchiveMaxBytes))
		}
		total += info.Size()
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = name
		header.Method = terminalZipMethod(name, info.Size())
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.CopyBuffer(writer, &io.LimitedReader{R: file, N: info.Size()}, make([]byte, terminalDownloadCopyBuffer))
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	closeErr := zw.Close()
	if walkErr != nil {
		return walkErr
	}
	return closeErr
}

func terminalZipMethod(name string, size int64) uint16 {
	// En un VPS pequeño, recomprimir archivos grandes puede monopolizar el único CPU.
	// ZIP Store conserva el empaquetado sin pagar ese coste para archivos >=128 MiB.
	if size >= 128<<20 {
		return zip.Store
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".zip", ".gz", ".tgz", ".bz2", ".xz", ".7z", ".rar", ".zst", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".mp3", ".aac", ".ogg", ".flac", ".mp4", ".mkv", ".webm", ".mov", ".pdf", ".apk", ".aab", ".jar", ".woff", ".woff2":
		return zip.Store
	default:
		return zip.Deflate
	}
}

type contextWriter struct {
	ctx context.Context
	w   io.Writer
}

func (w *contextWriter) Write(p []byte) (int, error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
		return w.w.Write(p)
	}
}

func (s *Server) createTerminalDownloadTicket(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		Target string `json:"target"`
		Path   string `json:"path"`
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Size   int64  `json:"size"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "solicitud de descarga invalida")
		return
	}
	ticket := terminalDownloadTicket{UserID: rs.User.ID, Target: strings.TrimSpace(req.Target), Path: strings.TrimSpace(req.Path), Name: safeTerminalDownloadName(req.Name, "descarga.bin"), Kind: strings.TrimSpace(req.Kind), Size: req.Size, ExpiresAt: time.Now().Add(terminalDownloadTicketTTL)}
	if ticket.Path == "" || len([]byte(ticket.Path)) > 4096 || strings.IndexByte(ticket.Path, 0) >= 0 {
		writeError(w, http.StatusBadRequest, "ruta de descarga invalida")
		return
	}
	if ticket.Target == "local" {
		target, err := inspectTerminalDownloadTarget(ticket.Path, false)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		ticket.Path, ticket.Name, ticket.Kind, ticket.Size = target.Path, target.Name, target.Kind, target.Size
	} else if strings.HasPrefix(ticket.Target, "agent:") {
		ticket.AgentID = strings.TrimSpace(strings.TrimPrefix(ticket.Target, "agent:"))
		if ticket.AgentID == "" {
			writeError(w, http.StatusBadRequest, "cliente de sistema invalido")
			return
		}
		agent, err := s.store.AgentByID(ticket.AgentID)
		if err != nil || !agent.Enabled || !agent.Online {
			writeError(w, http.StatusBadRequest, "cliente de sistema no disponible")
			return
		}
		if !s.hub.AgentSupports(ticket.AgentID, AgentCapabilityTerminalDownloadV1) {
			writeError(w, http.StatusConflict, "actualiza pangolite-client para habilitar descargas desde la terminal")
			return
		}
		if ticket.Kind != "file" && ticket.Kind != "directory" {
			writeError(w, http.StatusBadRequest, "tipo de descarga invalido")
			return
		}
	} else {
		writeError(w, http.StatusBadRequest, "destino de terminal invalido")
		return
	}
	token, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo preparar la descarga")
		return
	}
	ticket.Token = token
	if err := s.storeTerminalDownloadTicket(ticket); err != nil {
		writeError(w, http.StatusTooManyRequests, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "url": "/api/terminal/downloads/" + token, "name": ticket.Name, "kind": ticket.Kind})
}

func (s *Server) storeTerminalDownloadTicket(ticket terminalDownloadTicket) error {
	s.terminalDownloadsMu.Lock()
	defer s.terminalDownloadsMu.Unlock()
	now := time.Now()
	for token, existing := range s.terminalDownloads {
		if now.After(existing.ExpiresAt) {
			delete(s.terminalDownloads, token)
		}
	}
	if len(s.terminalDownloads) >= terminalDownloadMaxTickets {
		return errors.New("hay demasiadas descargas pendientes; espera a que terminen")
	}
	s.terminalDownloads[ticket.Token] = ticket
	return nil
}

func (s *Server) takeTerminalDownloadTicket(token string, userID int64) (terminalDownloadTicket, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return terminalDownloadTicket{}, false
	}
	s.terminalDownloadsMu.Lock()
	defer s.terminalDownloadsMu.Unlock()
	ticket, ok := s.terminalDownloads[token]
	if !ok || ticket.UserID != userID || time.Now().After(ticket.ExpiresAt) {
		if ok && time.Now().After(ticket.ExpiresAt) {
			delete(s.terminalDownloads, token)
		}
		return terminalDownloadTicket{}, false
	}
	delete(s.terminalDownloads, token)
	return ticket, true
}

func (s *Server) downloadTerminalTicket(w http.ResponseWriter, r *http.Request, rs requestSession) {
	select {
	case s.terminalDownloadSlots <- struct{}{}:
		defer func() { <-s.terminalDownloadSlots }()
	default:
		writeError(w, http.StatusTooManyRequests, "ya hay demasiadas descargas de terminal activas")
		return
	}
	ticket, ok := s.takeTerminalDownloadTicket(r.PathValue("token"), rs.User.ID)
	if !ok {
		writeError(w, http.StatusNotFound, "descarga expirada o ya utilizada")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if ticket.AgentID == "" {
		s.serveLocalTerminalDownload(w, r, rs, ticket)
		return
	}
	s.serveRemoteTerminalDownload(w, r, rs, ticket)
}

func (s *Server) serveLocalTerminalDownload(w http.ResponseWriter, r *http.Request, rs requestSession, ticket terminalDownloadTicket) {
	target, err := inspectTerminalDownloadTarget(ticket.Path, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	setTerminalDownloadHeaders(w, target.Name, target.Kind, target.Size)
	if err := writeTerminalDownloadPayload(r.Context(), w, target); err != nil {
		if s.log != nil {
			s.log.Warn("descarga local de terminal interrumpida", "user", rs.User.Username, "path", target.Path, "error", err.Error())
		}
		return
	}
	s.recordAudit(r, rs, "terminal.download", "terminal", "local", "", map[string]any{"path": target.Path, "kind": target.Kind})
}

func (s *Server) serveRemoteTerminalDownload(w http.ResponseWriter, r *http.Request, rs requestSession, ticket terminalDownloadTicket) {
	agent, err := s.store.AgentByID(ticket.AgentID)
	if err != nil || !agent.Enabled || !agent.Online {
		writeError(w, http.StatusBadRequest, "cliente de sistema no disponible")
		return
	}
	streamID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo crear el stream de descarga")
		return
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	stopCancelClose := context.AfterFunc(r.Context(), func() {
		_ = left.Close()
		_ = right.Close()
	})
	defer stopCancelClose()
	job := AgentStreamJob{ID: streamID, Mode: AgentStreamModeTerminalDownload, Path: ticket.Path}
	if _, err := s.hub.StartStream(r.Context(), ticket.AgentID, job, left); err != nil {
		if r.Context().Err() == nil {
			writeError(w, http.StatusServiceUnavailable, "el cliente no pudo iniciar la descarga")
		}
		return
	}
	reader := bufio.NewReaderSize(right, terminalDownloadHeaderMax)
	meta, err := readTerminalDownloadStreamHeader(reader)
	if err != nil {
		writeError(w, http.StatusBadGateway, "respuesta de descarga invalida del cliente")
		return
	}
	if meta.Error != "" {
		writeError(w, http.StatusBadRequest, meta.Error)
		return
	}
	if meta.Kind != "file" && meta.Kind != "directory" {
		writeError(w, http.StatusBadGateway, "tipo de descarga invalido desde el cliente")
		return
	}
	setTerminalDownloadHeaders(w, safeTerminalDownloadName(meta.Name, ticket.Name), meta.Kind, meta.Size)
	_, copyErr := io.CopyBuffer(w, reader, make([]byte, terminalDownloadCopyBuffer))
	if copyErr != nil && r.Context().Err() == nil && s.log != nil {
		s.log.Warn("descarga remota de terminal interrumpida", "user", rs.User.Username, "agent", ticket.AgentID, "path", ticket.Path, "error", copyErr.Error())
	}
	if copyErr == nil {
		s.recordAudit(r, rs, "terminal.download", "agent", ticket.AgentID, agent.ProjectID, map[string]any{"path": ticket.Path, "kind": meta.Kind})
	}
}

func setTerminalDownloadHeaders(w http.ResponseWriter, name, kind string, size int64) {
	name = safeTerminalDownloadName(name, "descarga.bin")
	contentType := "application/octet-stream"
	if kind == "directory" {
		contentType = "application/zip"
	} else if detected := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); detected != "" {
		contentType = detected
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	if kind == "file" && size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
}

func safeTerminalDownloadName(name, fallback string) string {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	name = filepath.Base(name)
	name = strings.Map(func(r rune) rune {
		if r == 0 || r == '\r' || r == '\n' || r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = fallback
	}
	if len([]byte(name)) > 240 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		for len([]byte(base+ext)) > 240 && len(base) > 1 {
			base = base[:len(base)-1]
		}
		name = base + ext
	}
	return name
}

func writeTerminalDownloadStreamHeader(w io.Writer, meta terminalDownloadStreamMeta) error {
	payload, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if len(payload) > terminalDownloadHeaderMax/2 {
		return errors.New("metadata de descarga demasiado grande")
	}
	header := append([]byte(nil), terminalDownloadStreamPrefix...)
	header = strconv.AppendInt(header, int64(len(payload)), 10)
	header = append(header, '\n')
	if err := writeFull(w, header); err != nil {
		return err
	}
	return writeFull(w, payload)
}

func readTerminalDownloadStreamHeader(reader *bufio.Reader) (terminalDownloadStreamMeta, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil {
		return terminalDownloadStreamMeta{}, err
	}
	line = bytesTrimSpaceNewline(line)
	if !strings.HasPrefix(string(line), string(terminalDownloadStreamPrefix)) {
		return terminalDownloadStreamMeta{}, errors.New("prefijo de descarga invalido")
	}
	lengthText := strings.TrimSpace(string(line[len(terminalDownloadStreamPrefix):]))
	n, err := strconv.Atoi(lengthText)
	if err != nil || n < 2 || n > terminalDownloadHeaderMax/2 {
		return terminalDownloadStreamMeta{}, errors.New("longitud de metadata invalida")
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return terminalDownloadStreamMeta{}, err
	}
	var meta terminalDownloadStreamMeta
	if err := json.Unmarshal(payload, &meta); err != nil {
		return terminalDownloadStreamMeta{}, err
	}
	return meta, nil
}

func bytesTrimSpaceNewline(data []byte) []byte {
	for len(data) > 0 && (data[len(data)-1] == '\n' || data[len(data)-1] == '\r') {
		data = data[:len(data)-1]
	}
	return data
}

func handleAgentTerminalDownloadStream(ctx context.Context, client *http.Client, base, configuredServer, fallback string, cfg AgentClientConfig, job AgentStreamJob, logger *slog.Logger) {
	wsURL, err := agentWebSocketURL(base, "/api/agent/streams/"+url.PathEscape(job.ID))
	if err != nil {
		logger.Warn("url de descarga de terminal invalida", "stream", job.ID, "error", err.Error())
		return
	}
	header := http.Header{}
	setAgentAuthHeaderWithEndpoint(header, cfg, configuredServer, fallback)
	ws, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header, HTTPClient: client})
	if err != nil {
		logger.Warn("websocket de descarga de terminal fallo", "stream", job.ID, "error", err.Error())
		return
	}
	defer ws.CloseNow()
	readCtx, readCancel := context.WithCancel(ctx)
	defer readCancel()
	go func() {
		for {
			if _, _, err := ws.Read(readCtx); err != nil {
				return
			}
		}
	}()
	writer := bufio.NewWriterSize(&terminalDownloadWSWriter{ctx: ctx, ws: ws}, terminalDownloadCopyBuffer)
	target, inspectErr := inspectTerminalDownloadTarget(job.Path, true)
	if inspectErr != nil {
		_ = writeTerminalDownloadStreamHeader(writer, terminalDownloadStreamMeta{Error: inspectErr.Error()})
		_ = writer.Flush()
		return
	}
	if err := writeTerminalDownloadStreamHeader(writer, terminalDownloadStreamMeta{Name: target.Name, Kind: target.Kind, Size: target.Size}); err != nil {
		logger.Debug("metadata de descarga no pudo enviarse", "stream", job.ID, "error", err.Error())
		return
	}
	if err := writeTerminalDownloadPayload(ctx, writer, target); err != nil {
		logger.Debug("descarga de terminal interrumpida", "stream", job.ID, "path", target.Path, "error", err.Error())
		return
	}
	if err := writer.Flush(); err != nil {
		logger.Debug("descarga de terminal no pudo finalizar", "stream", job.ID, "error", err.Error())
		return
	}
	_ = ws.Close(websocket.StatusNormalClosure, "")
}

type terminalDownloadWSWriter struct {
	ctx context.Context
	ws  *websocket.Conn
}

func (w *terminalDownloadWSWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if err := w.ws.Write(w.ctx, websocket.MessageBinary, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func formatTerminalDownloadSize(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	v := float64(value)
	for _, unit := range units {
		v /= 1024
		if v < 1024 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", v, unit)
		}
	}
	return fmt.Sprintf("%d B", value)
}
