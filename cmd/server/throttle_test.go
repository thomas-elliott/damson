package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/handlers/api"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// The login and bootstrap throttles key on the client IP, so the IP must not
// be caller-chosen. httptest requests arrive from 192.0.2.1.
func TestAuthThrottles_KeyOnTheClientIP(t *testing.T) {
	cases := []struct {
		name    string
		trusted []string
		// sameBucket: whether requests with different X-Forwarded-For values
		// share one throttle bucket.
		sameBucket bool
	}{
		{"untrusted caller forging X-Forwarded-For", nil, true},
		{"trusted proxy forwarding real clients", []string{"192.0.2.0/24"}, false},
	}
	for _, path := range []string{"/api/auth/login", "/api/auth/bootstrap"} {
		for _, tc := range cases {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				db := testdb.NewPostgres(t)
				local, err := auth.NewLocalAuthenticator(db, []byte("test-signing-key-at-least-thirty-two-bytes"))
				if err != nil {
					t.Fatalf("local auth: %v", err)
				}
				r, err := newRouter("", tc.trusted)
				if err != nil {
					t.Fatalf("router: %v", err)
				}
				h := api.New(api.Deps{DB: db, AuthMode: config.AuthModeLocal, Local: local, LoginThrottle: auth.NewLoginThrottle(1, 0)})
				api.AddRoutes(r.Group("/api"), auth.Middleware(local, nil), h)

				// A short password fails after the throttle has counted the
				// attempt, so neither route succeeds and only the throttle
				// decides between 4xx and 429. Distinct emails keep the
				// per-email bucket out of it.
				attempt := func(email, forwardedFor string) int {
					body := `{"email":"` + email + `","password":"x"}`
					req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-Forwarded-For", forwardedFor)
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					return w.Code
				}
				if code := attempt("a@example.com", "203.0.113.1"); code == http.StatusTooManyRequests {
					t.Fatal("first attempt was throttled")
				}
				code := attempt("b@example.com", "203.0.113.2")
				if throttled := code == http.StatusTooManyRequests; throttled != tc.sameBucket {
					t.Fatalf("second client got %d; want shared bucket = %v", code, tc.sameBucket)
				}
			})
		}
	}
}
