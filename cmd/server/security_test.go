package main

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// cspFor runs the securityHeaders middleware once and returns the emitted
// Content-Security-Policy header.
func cspFor(oidcOrigin string) string {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	securityHeaders(oidcOrigin)(c)
	return w.Header().Get("Content-Security-Policy")
}

// TestCSPAllowsBlobImages pins the directives the file-preview feature relies
// on. Inline image previews fetch the file as an authenticated blob and render
// it from a blob: object URL, so img-src MUST keep blob:. This guards against a
// future CSP edit silently breaking previews.
func TestCSPAllowsBlobImages(t *testing.T) {
	csp := cspFor("")
	if !strings.Contains(csp, "img-src 'self' data: blob:") {
		t.Fatalf("CSP must allow blob: images for inline previews; got %q", csp)
	}
	// Audio (<audio src=blob:>) and font specimens (@font-face from a blob URL)
	// both fetch bytes with auth and render from blob: object URLs.
	if !strings.Contains(csp, "media-src 'self' blob:") {
		t.Fatalf("CSP must allow blob: media for audio previews; got %q", csp)
	}
	if !strings.Contains(csp, "font-src 'self' data: blob:") {
		t.Fatalf("CSP must allow blob: fonts for font specimens; got %q", csp)
	}
	// pdf.js image codecs and the glTF Draco decoder compile WebAssembly.
	if !strings.Contains(csp, "script-src 'self' 'wasm-unsafe-eval'") {
		t.Fatalf("CSP must allow wasm compilation for pdf.js / Draco; got %q", csp)
	}
	if !strings.Contains(csp, "frame-ancestors") {
		t.Fatalf("CSP regression: frame-ancestors directive missing; got %q", csp)
	}
}

// TestCSPWhitelistsOIDCOrigin verifies the IdP origin lands in both connect-src
// (token/jwks fetches) and frame-src (hidden-iframe silent renew) when OIDC is
// configured, and stays absent in local mode.
func TestCSPWhitelistsOIDCOrigin(t *testing.T) {
	const idp = "https://idp.example.com"
	csp := cspFor(idp)
	if !strings.Contains(csp, "connect-src 'self' blob: "+idp) {
		t.Errorf("OIDC origin missing from connect-src; got %q", csp)
	}
	if !strings.Contains(csp, "frame-src 'self' "+idp) {
		t.Errorf("OIDC origin missing from frame-src; got %q", csp)
	}

	local := cspFor("")
	if strings.Contains(local, idp) {
		t.Errorf("local-mode CSP must not whitelist any IdP origin; got %q", local)
	}
}
