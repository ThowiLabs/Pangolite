package app

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestPrepareBrandingBackendRequestForcesIdentityAndClearsValidators(t *testing.T) {
	resource := Resource{Mode: ModeHTTP, BrandingLoaderEnabled: true}
	req := httptest.NewRequest(http.MethodGet, "https://app.example.com/", nil)
	req.Header.Set("Accept", "text/html")
	header := http.Header{
		"Accept-Encoding":   {"gzip, br"},
		"If-None-Match":     {`"v1"`},
		"If-Modified-Since": {"Wed, 21 Oct 2015 07:28:00 GMT"},
	}
	prepareBrandingBackendRequest(header, req, resource)
	if got := header.Get("Accept-Encoding"); got != "identity" {
		t.Fatalf("Accept-Encoding = %q, want identity", got)
	}
	if header.Get("If-None-Match") != "" || header.Get("If-Modified-Since") != "" {
		t.Fatalf("validadores condicionales no eliminados: %v", header)
	}
}

func TestPrepareBrandedHTMLResponseInjectsLoader(t *testing.T) {
	resource := Resource{Mode: ModeHTTP, PathPrefix: "/portal", BrandingLoaderEnabled: true}
	req := httptest.NewRequest(http.MethodGet, "https://app.example.com/portal/", nil)
	req.Host = "app.example.com"
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp := &http.Response{
		StatusCode:    http.StatusOK,
		Header:        http.Header{"Content-Type": {"text/html; charset=utf-8"}, "Content-Length": {"84"}, "ETag": {`"backend-v1"`}, "Last-Modified": {"Wed, 21 Oct 2015 07:28:00 GMT"}, "Cache-Control": {"private, max-age=300"}},
		Body:          io.NopCloser(strings.NewReader(`<!doctype html><html><head><title>App</title></head><body><main>Contenido</main></body></html>`)),
		ContentLength: 84,
	}
	if !prepareBrandedHTMLResponse(resp, req, resource) {
		t.Fatal("se esperaba inyeccion de branding")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, expected := range []string{"data-pangolite-branding-loader=\"1\"", "Thowilabs", "/portal/.pangolite/branding/loader.css?v=1", "/portal/.pangolite/branding/logo.png?v=1", "<main>Contenido</main>"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("falta %q en HTML inyectado: %s", expected, text)
		}
	}
	if resp.ContentLength != -1 || resp.Header.Get("Content-Length") != "" || resp.Header.Get("ETag") != "" || resp.Header.Get("Last-Modified") != "" {
		t.Fatalf("cabeceras de cuerpo transformado no se limpiaron: contentLength=%d header=%v", resp.ContentLength, resp.Header)
	}
	if cacheControl := strings.ToLower(resp.Header.Get("Cache-Control")); !strings.Contains(cacheControl, "private") || !strings.Contains(cacheControl, "no-cache") {
		t.Fatalf("HTML transformado debe conservar politica existente y forzar revalidacion: %q", cacheControl)
	}
}

func TestPrepareBrandedHTMLResponseRespectsStrictCSP(t *testing.T) {
	resource := Resource{Mode: ModeHTTP, PathPrefix: "/", BrandingLoaderEnabled: true}
	req := httptest.NewRequest(http.MethodGet, "https://secure.example.com/", nil)
	req.Host = "secure.example.com"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":            {"text/html; charset=utf-8"},
			"Content-Security-Policy": {"default-src 'self'; style-src 'nonce-AbCd1234'"},
		},
		Body: io.NopCloser(strings.NewReader(`<html><body>Seguro</body></html>`)),
	}
	if prepareBrandedHTMLResponse(resp, req, resource) {
		t.Fatal("un CSP que no permite estilos self debe omitir la inyeccion")
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "pangolite-brand-loader") {
		t.Fatal("el cuerpo no debe modificarse si CSP lo impide")
	}
}

func TestPrepareBrandedHTMLResponseRespectsCSPPath(t *testing.T) {
	resource := Resource{Mode: ModeHTTP, PathPrefix: "/portal", BrandingLoaderEnabled: true}
	req := httptest.NewRequest(http.MethodGet, "https://secure.example.com/portal/", nil)
	req.Host = "secure.example.com"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":            {"text/html; charset=utf-8"},
			"Content-Security-Policy": {"default-src 'self'; style-src https://secure.example.com/assets/"},
		},
		Body: io.NopCloser(strings.NewReader(`<html><body>Seguro</body></html>`)),
	}
	if prepareBrandedHTMLResponse(resp, req, resource) {
		t.Fatal("una fuente CSP limitada a otra ruta no debe aceptar el asset de branding")
	}
}

func TestPrepareBrandedHTMLResponseRespectsCSPPort(t *testing.T) {
	resource := Resource{Mode: ModeHTTP, PathPrefix: "/", BrandingLoaderEnabled: true}
	req := httptest.NewRequest(http.MethodGet, "https://secure.example.com:8443/", nil)
	req.Host = "secure.example.com:8443"
	req.Header.Set("Accept", "text/html")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":            {"text/html; charset=utf-8"},
			"Content-Security-Policy": {"style-src https://secure.example.com"},
		},
		Body: io.NopCloser(strings.NewReader(`<html><body>Seguro</body></html>`)),
	}
	if prepareBrandedHTMLResponse(resp, req, resource) {
		t.Fatal("una fuente CSP en el puerto por defecto no debe autorizar un origen :8443")
	}
}

func TestBrandingParserIgnoresBodyTextInsideScript(t *testing.T) {
	html := []byte(`<html><head><script>const sample = "<body class='fake'>";</script></head><body class="real"><main>OK</main></body></html>`)
	_, bodyEnd := findBrandingHTMLInsertionPoints(html)
	actual := strings.Index(string(html), `<body class="real">`) + len(`<body class="real">`)
	if bodyEnd != actual {
		t.Fatalf("body detectado en posicion incorrecta: got=%d want=%d", bodyEnd, actual)
	}
}

func TestPrepareBrandedHTMLResponseSkipsNonDocuments(t *testing.T) {
	resource := Resource{Mode: ModeHTTP, BrandingLoaderEnabled: true}
	req := httptest.NewRequest(http.MethodGet, "https://api.example.com/data", nil)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Accept", "application/json")
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}
	if prepareBrandedHTMLResponse(resp, req, resource) {
		t.Fatal("JSON/fetch no debe recibir branding")
	}

	wildcard := httptest.NewRequest(http.MethodGet, "https://api.example.com/error", nil)
	wildcard.Header.Set("Accept", "*/*")
	if brandingRequestCandidate(wildcard, resource) {
		t.Fatal("Accept */* sin navegacion HTML explicita no debe activar branding")
	}

	ranged := httptest.NewRequest(http.MethodGet, "https://app.example.com/", nil)
	ranged.Header.Set("Accept", "text/html")
	ranged.Header.Set("Range", "bytes=0-1023")
	if brandingRequestCandidate(ranged, resource) {
		t.Fatal("requests Range no deben transformarse")
	}
}

func TestBrandingResourceGatewayProxiesAndServesAssets(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html><html><head><title>Interna</title></head><body><h1>Aplicacion</h1></body></html>`)
	}))
	defer backend.Close()
	host, portText, err := net.SplitHostPort(strings.TrimPrefix(backend.URL, "http://"))
	if err != nil {
		t.Fatalf("backend sin host/puerto valido: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}

	server, store := testServerWithStore(t)
	project, err := store.AddProject(Project{Name: "Proyecto Branding"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.AddResource(Resource{
		ProjectID:             project.ID,
		Name:                  "Intranet",
		Mode:                  ModeHTTP,
		Domain:                "brand.example.com",
		PathPrefix:            "/portal",
		BackendScheme:         "http",
		BackendHost:           host,
		BackendPort:           port,
		OriginType:            OriginLocal,
		Enabled:               true,
		BrandingLoaderEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created.BrandingLoaderEnabled {
		t.Fatal("branding no persistido al crear")
	}

	req := httptest.NewRequest(http.MethodGet, "https://brand.example.com/portal/", nil)
	req.Host = "brand.example.com"
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Sec-Fetch-Dest", "document")
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status inesperado: %d body=%q", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "data-pangolite-branding-loader=\"1\"") || !strings.Contains(rr.Body.String(), "Aplicacion") {
		t.Fatalf("respuesta sin branding o sin backend: %s", rr.Body.String())
	}

	assetReq := httptest.NewRequest(http.MethodGet, "https://brand.example.com/portal/.pangolite/branding/loader.css?v=1", nil)
	assetReq.Host = "brand.example.com"
	assetRR := httptest.NewRecorder()
	server.Handler().ServeHTTP(assetRR, assetReq)
	if assetRR.Code != http.StatusOK || !strings.Contains(assetRR.Header().Get("Content-Type"), "text/css") || !strings.Contains(assetRR.Body.String(), "pangolite-brand-loader") {
		t.Fatalf("asset de branding invalido: status=%d headers=%v body=%q", assetRR.Code, assetRR.Header(), assetRR.Body.String())
	}
}
