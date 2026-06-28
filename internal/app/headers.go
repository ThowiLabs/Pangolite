package app

import (
	"net/http"
	"strings"
)

var hopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"TE":                  true,
	"Trailer":             true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

func cloneSafeHeader(src http.Header) http.Header {
	dst := http.Header{}
	for k, vals := range src {
		if hopHeaders[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
	return dst
}

func copySafeHeader(dst, src http.Header) {
	for k, vals := range src {
		if hopHeaders[http.CanonicalHeaderKey(k)] {
			continue
		}
		for _, v := range vals {
			dst.Add(k, v)
		}
	}
}

func cloneProxyRequestHeader(src http.Header) http.Header {
	dst := cloneSafeHeader(src)
	stripInternalProxyHeaders(dst)
	return dst
}

func stripInternalProxyHeaders(h http.Header) {
	h.Del("X-CSRF-Token")
	for key := range h {
		if strings.HasPrefix(strings.ToLower(key), "x-pangolite-") {
			h.Del(key)
		}
	}
	filterInternalCookies(h)
}

func filterInternalCookies(h http.Header) {
	values := h.Values("Cookie")
	if len(values) == 0 {
		return
	}
	h.Del("Cookie")
	kept := []string{}
	for _, line := range values {
		for _, part := range strings.Split(line, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name, _, _ := strings.Cut(part, "=")
			name = strings.TrimSpace(name)
			if name == sessionCookieName || strings.HasPrefix(name, "pangolite_resource_") {
				continue
			}
			kept = append(kept, part)
		}
	}
	if len(kept) > 0 {
		h.Set("Cookie", strings.Join(kept, "; "))
	}
}
