package app

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	resourceBrandingAssetRoot = "/.pangolite/branding/"
	resourceBrandingCSSFile   = "loader.css"
	resourceBrandingLogoFile  = "logo.png"
	resourceBrandingVersion   = "1"
	resourceBrandingProbeMax  = 128 << 10
)

const resourceBrandingCSS = `#pangolite-brand-loader[data-pangolite-branding-loader="1"]{position:fixed;inset:0;z-index:2147483647;display:grid;place-items:center;overflow:hidden;background:radial-gradient(circle at 50% 42%,#23120c 0,#0d0d10 42%,#070709 100%);color:#fff;font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;opacity:1;visibility:visible;pointer-events:auto;animation:pangolite-brand-exit .38s ease 1.15s forwards}#pangolite-brand-loader .pangolite-brand-loader__inner{display:grid;justify-items:center;gap:18px;transform:translateY(-2vh)}#pangolite-brand-loader .pangolite-brand-loader__mark{position:relative;width:104px;height:104px;display:grid;place-items:center}#pangolite-brand-loader .pangolite-brand-loader__mark:before{content:"";position:absolute;inset:-9px;border-radius:999px;border:2px solid rgba(255,255,255,.12);border-top-color:#ff7a18;border-right-color:#ff3d2e;animation:pangolite-brand-spin .9s linear infinite}#pangolite-brand-loader img{display:block;width:88px;height:88px;object-fit:contain;filter:drop-shadow(0 12px 26px rgba(255,70,20,.22))}#pangolite-brand-loader .pangolite-brand-loader__name{font-size:16px;font-weight:800;line-height:1;letter-spacing:.32em;padding-left:.32em;text-transform:uppercase;text-shadow:0 7px 24px rgba(0,0,0,.55)}#pangolite-brand-loader .pangolite-brand-loader__line{width:42px;height:2px;border-radius:999px;background:linear-gradient(90deg,#ff9a1f,#ff3d2e);box-shadow:0 0 18px rgba(255,81,30,.34);animation:pangolite-brand-pulse .9s ease-in-out infinite alternate}@keyframes pangolite-brand-spin{to{transform:rotate(360deg)}}@keyframes pangolite-brand-pulse{from{opacity:.45;transform:scaleX(.7)}to{opacity:1;transform:scaleX(1.25)}}@keyframes pangolite-brand-exit{to{opacity:0;visibility:hidden;pointer-events:none}}@media(prefers-reduced-motion:reduce){#pangolite-brand-loader[data-pangolite-branding-loader="1"]{animation-duration:.01ms;animation-delay:.8s}#pangolite-brand-loader .pangolite-brand-loader__mark:before,#pangolite-brand-loader .pangolite-brand-loader__line{animation:none}}`

func resourceBrandingAssetPath(resource Resource, file string) string {
	prefix := strings.TrimSpace(resource.PathPrefix)
	if prefix == "" || prefix == "/" {
		return resourceBrandingAssetRoot + file
	}
	prefix = strings.TrimSuffix(prefix, "/")
	return prefix + resourceBrandingAssetRoot + file
}

func resourceBrandingAsset(resource Resource, requestPath string) string {
	if !resource.BrandingLoaderEnabled || resource.Mode != ModeHTTP || resource.RedirectEnabled {
		return ""
	}
	switch requestPath {
	case resourceBrandingAssetPath(resource, resourceBrandingCSSFile):
		return resourceBrandingCSSFile
	case resourceBrandingAssetPath(resource, resourceBrandingLogoFile):
		return resourceBrandingLogoFile
	default:
		return ""
	}
}

func (s *Server) serveResourceBrandingAsset(w http.ResponseWriter, r *http.Request, resource Resource) bool {
	asset := resourceBrandingAsset(resource, r.URL.Path)
	if asset == "" {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch asset {
	case resourceBrandingCSSFile:
		body := []byte(resourceBrandingCSS)
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(body)
		}
		return true
	case resourceBrandingLogoFile:
		body, err := assetsFS.ReadFile("assets/app/logo-mark.png")
		if err != nil {
			http.NotFound(w, r)
			return true
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			_, _ = w.Write(body)
		}
		return true
	default:
		return false
	}
}

func brandingRequestCandidate(r *http.Request, resource Resource) bool {
	if r == nil || !resource.BrandingLoaderEnabled || resource.Mode != ModeHTTP || resource.RedirectEnabled || r.Method != http.MethodGet {
		return false
	}
	if strings.TrimSpace(r.Header.Get("Range")) != "" {
		return false
	}
	if dest := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Dest"))); dest != "" && dest != "document" {
		return false
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	if !strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/xhtml+xml") {
		return false
	}
	return true
}

func prepareBrandingBackendRequest(h http.Header, original *http.Request, resource Resource) {
	if !brandingRequestCandidate(original, resource) {
		return
	}
	// La inyeccion cambia el cuerpo. Pedir identidad evita tener que recomprimir
	// contenido de terceros y quitar validadores evita respuestas 304 sin HTML.
	h.Set("Accept-Encoding", "identity")
	h.Del("If-None-Match")
	h.Del("If-Modified-Since")
}

func prepareBrandedHTMLResponse(resp *http.Response, request *http.Request, resource Resource) bool {
	if resp == nil || resp.Body == nil || !brandingRequestCandidate(request, resource) || resp.StatusCode != http.StatusOK {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "text/html") {
		return false
	}
	if strings.TrimSpace(resp.Header.Get("Content-Range")) != "" {
		return false
	}
	if disposition := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Disposition"))); strings.Contains(disposition, "attachment") {
		return false
	}
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if encoding != "" && encoding != "identity" {
		return false
	}
	cssPath := resourceBrandingAssetPath(resource, resourceBrandingCSSFile)
	if !cspAllowsBrandingStyles(resp.Header.Values("Content-Security-Policy"), request, cssPath) {
		return false
	}

	cssURL := cssPath + "?v=" + resourceBrandingVersion
	logoURL := resourceBrandingAssetPath(resource, resourceBrandingLogoFile) + "?v=" + resourceBrandingVersion
	body, injected := newBrandingInjectReadCloser(resp.Body, cssURL, logoURL)
	resp.Body = body
	if !injected {
		return false
	}
	resp.ContentLength = -1
	for _, name := range []string{"Content-Length", "Content-MD5", "Digest", "ETag", "Last-Modified", "Accept-Ranges"} {
		resp.Header.Del(name)
	}
	ensureBrandingResponseRevalidation(resp.Header)
	return true
}

func ensureBrandingResponseRevalidation(h http.Header) {
	values := h.Values("Cache-Control")
	for _, value := range values {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "no-store") || strings.Contains(lower, "no-cache") {
			return
		}
	}
	if len(values) == 0 {
		h.Set("Cache-Control", "no-cache")
		return
	}
	h.Set("Cache-Control", strings.Join(values, ", ")+", no-cache")
}

func cspAllowsBrandingStyles(values []string, request *http.Request, cssPath string) bool {
	if len(values) == 0 {
		return true
	}
	scheme, host, port := brandingRequestOrigin(request)
	for _, value := range values {
		for _, policy := range strings.Split(value, ",") {
			if !cspPolicyAllowsSelfStyle(policy, scheme, host, port, cssPath) {
				return false
			}
		}
	}
	return true
}

func cspPolicyAllowsSelfStyle(policy, scheme, host, port, cssPath string) bool {
	directives := map[string][]string{}
	for _, raw := range strings.Split(policy, ";") {
		fields := strings.Fields(strings.TrimSpace(raw))
		if len(fields) == 0 {
			continue
		}
		name := strings.ToLower(fields[0])
		if _, exists := directives[name]; !exists {
			directives[name] = fields[1:]
		}
	}
	var sources []string
	var found bool
	for _, name := range []string{"style-src-elem", "style-src", "default-src"} {
		if candidate, ok := directives[name]; ok {
			sources = candidate
			found = true
			break
		}
	}
	if !found {
		return true
	}
	for _, source := range sources {
		source = strings.TrimSpace(strings.ToLower(source))
		switch source {
		case "'self'", "*":
			return true
		case "http:":
			if scheme == "http" {
				return true
			}
		case "https:":
			if scheme == "https" {
				return true
			}
		}
		if cspHostSourceMatchesSelf(source, scheme, host, port, cssPath) {
			return true
		}
	}
	return false
}

func brandingRequestOrigin(request *http.Request) (scheme, host, port string) {
	scheme = "https"
	if request != nil {
		scheme = publicSchemeForResource(request, Resource{})
		authority := strings.TrimSpace(request.Host)
		if h, p, err := net.SplitHostPort(authority); err == nil {
			host = strings.ToLower(strings.Trim(h, "[]"))
			port = p
		} else {
			host = strings.ToLower(strings.Trim(authority, "[]"))
		}
	}
	if port == "" {
		port = defaultBrandingPort(scheme)
	}
	return scheme, host, port
}

func defaultBrandingPort(scheme string) string {
	if strings.EqualFold(scheme, "http") {
		return "80"
	}
	return "443"
}

func cspHostSourceMatchesSelf(source, scheme, host, port, assetPath string) bool {
	if source == "" || host == "" || strings.HasPrefix(source, "'") {
		return false
	}
	candidate := source
	if strings.HasPrefix(candidate, "//") {
		candidate = scheme + ":" + candidate
	} else if !strings.Contains(candidate, "://") {
		candidate = scheme + "://" + candidate
	}
	u, err := url.Parse(candidate)
	if err != nil || !strings.EqualFold(u.Scheme, scheme) {
		return false
	}
	candidateHost := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if candidateHost != host && !(strings.HasPrefix(candidateHost, "*.") && strings.HasSuffix(host, candidateHost[1:])) {
		return false
	}
	candidatePort := u.Port()
	if candidatePort == "" {
		candidatePort = defaultBrandingPort(scheme)
	}
	if candidatePort != "*" && candidatePort != port {
		return false
	}
	path := u.EscapedPath()
	if path == "" || path == "/" {
		return true
	}
	if strings.HasSuffix(path, "/") {
		return strings.HasPrefix(assetPath, path)
	}
	return assetPath == path
}

type brandingInjectReadCloser struct {
	reader io.Reader
	closer io.Closer
}

func newBrandingInjectReadCloser(source io.ReadCloser, cssURL, logoURL string) (io.ReadCloser, bool) {
	buffered := bufio.NewReaderSize(source, 16<<10)
	prefix := make([]byte, 0, 16<<10)
	tmp := make([]byte, 4096)
	for len(prefix) < resourceBrandingProbeMax {
		remaining := resourceBrandingProbeMax - len(prefix)
		chunk := tmp
		if remaining < len(chunk) {
			chunk = chunk[:remaining]
		}
		n, err := buffered.Read(chunk)
		if n > 0 {
			prefix = append(prefix, chunk[:n]...)
		}
		_, bodyEnd := findBrandingHTMLInsertionPoints(prefix)
		if bodyEnd >= 0 || err != nil || n == 0 {
			break
		}
	}
	modified, injected := injectBrandingHTMLPrefix(prefix, cssURL, logoURL)
	return &brandingInjectReadCloser{reader: io.MultiReader(bytes.NewReader(modified), buffered), closer: source}, injected
}

func (r *brandingInjectReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *brandingInjectReadCloser) Close() error {
	return r.closer.Close()
}

func injectBrandingHTMLPrefix(prefix []byte, cssURL, logoURL string) ([]byte, bool) {
	if len(prefix) == 0 || bytes.Contains(bytes.ToLower(prefix), []byte(`data-pangolite-branding-loader=`)) {
		return prefix, false
	}
	link := `<link rel="stylesheet" href="` + html.EscapeString(cssURL) + `" data-pangolite-branding-style="1">`
	overlay := `<div id="pangolite-brand-loader" data-pangolite-branding-loader="1" role="status" aria-label="Cargando"><div class="pangolite-brand-loader__inner"><div class="pangolite-brand-loader__mark"><img src="` + html.EscapeString(logoURL) + `" alt=""></div><div class="pangolite-brand-loader__name">Thowilabs</div><div class="pangolite-brand-loader__line"></div></div></div>`

	headEnd, bodyEnd := findBrandingHTMLInsertionPoints(prefix)
	if bodyEnd < 0 {
		return prefix, false
	}

	type insertion struct {
		at   int
		text string
	}
	insertions := make([]insertion, 0, 2)
	if headEnd >= 0 {
		insertions = append(insertions, insertion{at: headEnd, text: link})
	} else {
		insertions = append(insertions, insertion{at: bodyEnd, text: link})
	}
	insertions = append(insertions, insertion{at: bodyEnd, text: overlay})
	if len(insertions) == 2 && insertions[0].at > insertions[1].at {
		insertions[0], insertions[1] = insertions[1], insertions[0]
	}

	var out bytes.Buffer
	out.Grow(len(prefix) + len(link) + len(overlay))
	cursor := 0
	for _, item := range insertions {
		if item.at < cursor {
			item.at = cursor
		}
		out.Write(prefix[cursor:item.at])
		out.WriteString(item.text)
		cursor = item.at
	}
	out.Write(prefix[cursor:])
	return out.Bytes(), true
}

func findBrandingHTMLInsertionPoints(content []byte) (headEnd, bodyEnd int) {
	headEnd, bodyEnd = -1, -1
	lower := bytes.ToLower(content)
	for i := 0; i < len(lower); {
		rel := bytes.IndexByte(lower[i:], '<')
		if rel < 0 {
			break
		}
		start := i + rel
		if bytes.HasPrefix(lower[start:], []byte("<!--")) {
			end := bytes.Index(lower[start+4:], []byte("-->"))
			if end < 0 {
				break
			}
			i = start + 4 + end + 3
			continue
		}
		if bytes.HasPrefix(lower[start:], []byte("<!")) || bytes.HasPrefix(lower[start:], []byte("<?")) {
			end := findHTMLTagEnd(content, start+2)
			if end < 0 {
				break
			}
			i = end
			continue
		}

		pos := start + 1
		closing := false
		if pos < len(lower) && lower[pos] == '/' {
			closing = true
			pos++
		}
		for pos < len(lower) && isHTMLSpace(lower[pos]) {
			pos++
		}
		nameStart := pos
		for pos < len(lower) && isHTMLTagNameByte(lower[pos]) {
			pos++
		}
		if nameStart == pos {
			i = start + 1
			continue
		}
		name := string(lower[nameStart:pos])
		end := findHTMLTagEnd(content, pos)
		if end < 0 {
			break
		}
		if !closing {
			switch name {
			case "head":
				if headEnd < 0 {
					headEnd = end
				}
			case "body":
				bodyEnd = end
				return headEnd, bodyEnd
			case "script", "style":
				closingNeedle := []byte("</" + name)
				closeRel := bytes.Index(lower[end:], closingNeedle)
				if closeRel < 0 {
					return headEnd, bodyEnd
				}
				closeStart := end + closeRel
				closeEnd := findHTMLTagEnd(content, closeStart+len(closingNeedle))
				if closeEnd < 0 {
					return headEnd, bodyEnd
				}
				i = closeEnd
				continue
			}
		}
		i = end
	}
	return headEnd, bodyEnd
}

func findHTMLTagEnd(content []byte, from int) int {
	var quote byte
	for i := from; i < len(content); i++ {
		c := content[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			continue
		}
		if c == '>' {
			return i + 1
		}
	}
	return -1
}

func isHTMLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

func isHTMLTagNameByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == ':'
}
