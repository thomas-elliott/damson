// Damson server entry point. Wires config, DB (with migrations), OIDC or
// local authentication, HTTP handlers, and the embedded SPA into a single
// process.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/handlers/api"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/queue"
	"github.com/thomas-elliott/damson/internal/services/audit"
	"github.com/thomas-elliott/damson/internal/services/imports"
	"github.com/thomas-elliott/damson/internal/services/metadata"
	"github.com/thomas-elliott/damson/internal/services/preview"
	"github.com/thomas-elliott/damson/internal/services/storage"
	damsonweb "github.com/thomas-elliott/damson/internal/web"
)

func main() {
	slog.SetDefault(newLogger())
	cfg := config.Load()

	if cfg.GinMode != "" {
		gin.SetMode(cfg.GinMode)
	}

	gormDB, err := db.Open(cfg.PostgresDSN)
	if err != nil {
		slog.Error("db open", "err", err)
		os.Exit(1)
	}

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	if err := db.Migrate(startupCtx, gormDB); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	var (
		authenticator auth.Authenticator
		localAuth     *auth.LocalAuthenticator
	)

	switch cfg.AuthMode {
	case config.AuthModeOIDC:
		oa, err := auth.NewOIDCAuthenticator(startupCtx, gormDB, auth.OIDCConfig{
			IssuerURL: cfg.OIDCIssuerURL,
			ClientID:  cfg.OIDCClientID,
			Audience:  cfg.OIDCAudience,
		})
		if err != nil {
			slog.Error("oidc init", "err", err)
			os.Exit(1)
		}
		authenticator = oa
		slog.Info("auth mode", "mode", "oidc", "issuer", cfg.OIDCIssuerURL)

	case config.AuthModeLocal:
		signingKey := cfg.JWTSigningKey
		if len(signingKey) == 0 {
			k, err := auth.LoadOrCreateJWTKey(cfg.JWTKeyFile)
			if err != nil {
				slog.Error("jwt key", "err", err)
				os.Exit(1)
			}
			signingKey = k
		}
		la, err := auth.NewLocalAuthenticatorWithIssuer(gormDB, signingKey, cfg.BaseURL)
		if err != nil {
			slog.Error("local auth init", "err", err)
			os.Exit(1)
		}
		if err := auth.SeedLocalAdmin(startupCtx, gormDB, cfg.LocalAdminEmail, cfg.LocalAdminPassword); err != nil {
			slog.Error("seed local admin", "err", err)
			os.Exit(1)
		}
		authenticator = la
		localAuth = la
		slog.Info("auth mode", "mode", "local")
	}

	store, err := storage.New(startupCtx, cfg)
	if err != nil {
		slog.Error("storage init", "err", err)
		os.Exit(1)
	}
	slog.Info("storage mode", "mode", cfg.StorageMode)

	previewBroadcaster := preview.NewBroadcaster()
	broadcaster := imports.NewBroadcaster()
	metaRegistry := metadata.Default(metadata.ClientOptions{})

	// River-backed job queue. queue.New wires the import→preview handoff through
	// River internally, so imports enqueue preview work on the preview queue.
	jobQueue, err := queue.New(startupCtx, queue.Config{
		DSN:               cfg.PostgresDSN,
		ImportWorkers:     cfg.ImportQueueWorkers,
		PreviewWorkers:    cfg.PreviewQueueWorkers,
		ImportJobTimeout:  cfg.ImportJobTimeout,
		PreviewJobTimeout: cfg.PreviewJobTimeout,
		ImportDeps:        imports.Deps{DB: gormDB, Storage: store, Broadcaster: broadcaster, Metadata: metaRegistry},
		PreviewDeps:       preview.Deps{DB: gormDB, Storage: store, Broadcaster: previewBroadcaster},
		Logger:            slog.Default(),
	})
	if err != nil {
		slog.Error("queue init", "err", err)
		os.Exit(1)
	}
	// Start with a long-lived context: cancelling Start's context soft-stops the
	// client, so shutdown is driven by Stop below, not by startupCtx expiring.
	if err := jobQueue.Start(context.Background()); err != nil {
		slog.Error("queue start", "err", err)
		os.Exit(1)
	}
	importEnqueuer := jobQueue.Imports()
	previewEnqueuer := jobQueue.Previews()
	slog.Info("job queue started", "import_workers", cfg.ImportQueueWorkers, "preview_workers", cfg.PreviewQueueWorkers)

	// Reconcile: re-enqueue any non-terminal domain row without a live job.
	// Idempotent (River dedupes by args), so it's safe every boot and covers the
	// one-time cutover of rows that were in flight before River existed. Previews
	// first — ResumeInFlight resets processing→queued for files a worker died on.
	if resumed, err := preview.ResumeInFlight(startupCtx, gormDB, previewEnqueuer); err != nil {
		slog.Warn("preview reconcile failed", "err", err)
	} else if resumed > 0 {
		slog.Info("preview reconcile", "enqueued", resumed)
	}
	if resumed, err := imports.ResumeInFlight(startupCtx, gormDB, importEnqueuer); err != nil {
		slog.Warn("import reconcile failed", "err", err)
	} else if resumed > 0 {
		slog.Info("import reconcile", "enqueued", resumed)
	}

	purgeCtx, cancelPurge := context.WithCancel(context.Background())
	defer cancelPurge()
	purger := &audit.Purger{DB: gormDB, Interval: cfg.AuditPurgeInterval}
	go purger.Run(purgeCtx)

	r := gin.New()
	r.Use(requestID())
	r.Use(requestLogger())
	r.Use(gin.CustomRecovery(recoveryHandler))
	r.Use(securityHeaders(oidcCSPOrigin(cfg)))
	r.Use(jsonBodyCap(maxJSONBodyBytes))

	apiGroup := r.Group("/api")
	h := api.New(api.Deps{
		DB:                 gormDB,
		AuthMode:           cfg.AuthMode,
		Local:              localAuth,
		Storage:            store,
		ImportPool:         importEnqueuer,
		Broadcaster:        broadcaster,
		PreviewPool:        previewEnqueuer,
		PreviewBroadcaster: previewBroadcaster,
		Metadata:           metaRegistry,
		OrphanMinAge:       cfg.OrphanMinAge,
		// Echoed to the SPA on /api/auth/mode in OIDC mode (empty in local).
		OIDCAuthority: cfg.OIDCIssuerURL,
		OIDCClientID:  cfg.OIDCClientID,
	})
	api.AddRoutes(apiGroup, auth.Middleware(authenticator, auth.NewTokenAuthenticator(gormDB)), h)

	if err := damsonweb.Mount(r); err != nil {
		slog.Error("mount spa", "err", err)
		os.Exit(1)
	}

	// Timeouts:
	//   ReadHeaderTimeout — defends against slow-loris on the request line.
	//   ReadTimeout       — covers the body too; uploads can be large so this
	//                       is generous. Per-handler control happens via
	//                       request context cancellation.
	//   WriteTimeout      — bounds long-running responses (file streams).
	//   IdleTimeout       — keeps keep-alive connections from accruing.
	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "addr", cfg.ListenAddr)
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-stop:
		slog.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cancelPurge()
	// Drain in-flight jobs before stopping the HTTP server. Anything still
	// running when ctx expires is abandoned; the next boot's reconcile picks up
	// whatever remained non-terminal.
	if err := jobQueue.Stop(shutdownCtx); err != nil {
		slog.Warn("queue stop", "err", err)
	}
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
}

// maxJSONBodyBytes caps the body Gin will read on JSON-shaped requests.
// Multipart uploads (POST /api/imports) are exempt — they handle their own
// streaming pipeline, see internal/handlers/api/assets.go. 1 MiB is well above
// any reasonable JSON payload (the largest is a licence's original_text) and
// well below the threshold at which a malicious client can OOM the process.
const maxJSONBodyBytes = 1 << 20

// securityHeaders applies a conservative set of response headers to every
// request. CSP locks the SPA to its own origin so a stored XSS in a served
// blob can't reach back into damson's UI. X-Frame-Options blocks clickjacking;
// X-Content-Type-Options stops the browser sniffing past our declared types.
// Referrer-Policy keeps internal paths out of upstream Referer headers.
// oidcCSPOrigin returns the scheme://host of the configured OIDC issuer, to be
// whitelisted in the CSP so the SPA can reach the IdP. Empty in local mode (no
// issuer) or if the issuer URL can't be parsed — in which case the CSP stays at
// the tighter 'self'-only baseline.
func oidcCSPOrigin(cfg config.Config) string {
	if cfg.OIDCIssuerURL == "" {
		return ""
	}
	u, err := url.Parse(cfg.OIDCIssuerURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func securityHeaders(oidcOrigin string) gin.HandlerFunc {
	// In OIDC mode the SPA talks to the identity provider's origin: it fetches
	// the discovery doc + token/userinfo endpoints (connect-src) and runs
	// silent token renewal in a hidden iframe that navigates to the IdP's
	// authorize endpoint (frame-src). Local mode makes no cross-origin calls,
	// so the IdP origin is only whitelisted when it's actually configured.
	// blob: in connect-src: wavesurfer fetch()es the audio's blob: object URL to
	// decode a waveform, and fetch() of a blob: URL is governed by connect-src.
	connectSrc := "connect-src 'self' blob:"
	frameSrc := "frame-src 'self'"
	if oidcOrigin != "" {
		connectSrc += " " + oidcOrigin
		frameSrc += " " + oidcOrigin
	}
	// blob: is needed across img/media/font because previews fetch file bytes
	// with auth and render them from blob: object URLs: images and derivative
	// posters (img-src), inline <audio> (media-src), and live @font-face
	// specimens (font-src).
	// 'wasm-unsafe-eval' lets WebAssembly compile without allowing arbitrary
	// eval: pdf.js uses WASM image codecs and the glTF viewer uses a Draco WASM
	// decoder. It is far narrower than 'unsafe-eval'.
	csp := "default-src 'self'; " +
		"script-src 'self' 'wasm-unsafe-eval'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data: blob:; " +
		"media-src 'self' blob:; " +
		"font-src 'self' data: blob:; " +
		connectSrc + "; " +
		frameSrc + "; " +
		"object-src 'none'; " +
		"base-uri 'self'; " +
		"frame-ancestors 'none';"
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", csp)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Next()
	}
}

// jsonBodyCap wraps the request body in http.MaxBytesReader for routes that
// don't stream a multipart upload. The exemption keeps POST /api/imports
// streaming-capable while every other endpoint gains a hard ceiling on body
// reads. 413 surfaces if the limit trips.
func jsonBodyCap(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := c.Request.Header.Get("Content-Type")
		if c.Request.Body != nil && !strings.HasPrefix(ct, "multipart/") {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

const (
	requestIDHeader = "X-Request-Id"
	requestIDKey    = "damson.request_id"

	// Mirror the unexported context keys set by the auth / api packages.
	userContextKey    = "damson.user"
	libraryContextKey = "damson.library"
)

// requestID honours an inbound X-Request-Id (so a correlation id set by a
// reverse proxy or the SPA survives) and otherwise mints one. It's stored on
// the context and echoed on the response so a client can quote it in a bug
// report.
func requestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(requestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		c.Set(requestIDKey, id)
		c.Header(requestIDHeader, id)
		c.Next()
	}
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}

// recoveryHandler converts a handler panic into a structured error log with a
// stack trace and attaches it to c.Errors so requestLogger surfaces it on the
// request line, then responds 500.
func recoveryHandler(c *gin.Context, recovered any) {
	err := fmt.Errorf("panic: %v", recovered)
	_ = c.Error(err)
	slog.Error("panic recovered",
		"err", err,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"request_id", c.GetString(requestIDKey),
		"stack", string(debug.Stack()),
	)
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// countingBody records how many bytes were read off the socket. Atomic
// because the server can still be draining the body as the chain unwinds.
type countingBody struct {
	io.ReadCloser
	n atomic.Int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.n.Add(int64(n))
	}
	return n, err
}

func (b *countingBody) read() int64 { return b.n.Load() }

// countBody must wrap the body before jsonBodyCap does, so the counter sits
// under MaxBytesReader and measures bytes read rather than bytes allowed.
func countBody(c *gin.Context) *countingBody {
	if c.Request.Body == nil || c.Request.Body == http.NoBody {
		return nil
	}
	b := &countingBody{ReadCloser: c.Request.Body}
	c.Request.Body = b
	return b
}

func isUploadStream(r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		return false
	}
	ct := r.Header.Get("Content-Type")
	return strings.HasPrefix(ct, "multipart/") || strings.HasPrefix(ct, "application/octet-stream")
}

func requestMessage(method, path string, status int, d time.Duration) string {
	return fmt.Sprintf("%s %s → %d in %dms", method, path, status, d.Milliseconds())
}

// requestLogger emits an slog line when an upload request starts and one when
// any request finishes. 5xx logs at error, 4xx at warn, everything else info.
// On 4xx/5xx it surfaces the cause(s) that handlers attached via
// respond400/respond500 (c.Errors). No request bodies, headers or query
// strings (don't leak tokens).
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		body := countBody(c)

		if isUploadStream(c.Request) {
			slog.Info(fmt.Sprintf("%s %s → upload started", c.Request.Method, c.Request.URL.Path),
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"phase", "start",
				"content_length", c.Request.ContentLength,
				"remote_ip", c.ClientIP(),
				"request_id", c.GetString(requestIDKey),
			)
		}

		c.Next()

		status := c.Writer.Status()
		elapsed := time.Since(start)
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"duration_ms", elapsed.Milliseconds(),
			"remote_ip", c.ClientIP(),
			"request_id", c.GetString(requestIDKey),
		}
		if n := c.Writer.Size(); n >= 0 {
			attrs = append(attrs, "bytes_out", n)
		}
		if body != nil {
			if n := body.read(); n > 0 {
				attrs = append(attrs, "bytes_in", n)
			}
		}
		if errors.Is(c.Request.Context().Err(), context.Canceled) {
			attrs = append(attrs, "client_aborted", true)
		}
		if u, ok := c.Get(userContextKey); ok {
			if user, ok := u.(*models.User); ok && user != nil {
				attrs = append(attrs, "user_id", user.ID)
			}
		}
		if l, ok := c.Get(libraryContextKey); ok {
			if lib, ok := l.(*models.Library); ok && lib != nil {
				attrs = append(attrs, "library_id", lib.ID)
			}
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "err", strings.Join(c.Errors.Errors(), "; "))
		}

		msg := requestMessage(c.Request.Method, c.Request.URL.Path, status, elapsed)
		switch {
		case status >= 500:
			slog.Error(msg, attrs...)
		case status >= 400:
			slog.Warn(msg, attrs...)
		default:
			slog.Info(msg, attrs...)
		}
	}
}

const (
	vlMessageKey = "_msg"
	vlTimeKey    = "_time"
)

func victoriaLogsKeys(groups []string, a slog.Attr) slog.Attr {
	// A group attribute called "msg" is data; renaming it would collide with
	// the record's own message.
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.MessageKey:
		a.Key = vlMessageKey
	case slog.TimeKey:
		a.Key = vlTimeKey
	}
	return a
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch os.Getenv("DAMSON_LOG_LEVEL") {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level, AddSource: true}
	var h slog.Handler
	if os.Getenv("DAMSON_LOG_FORMAT") == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		opts.ReplaceAttr = victoriaLogsKeys
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h).With("service", "damson", "version", readVersion())
}

// readVersion returns a best-effort build identifier for the version log
// field: the module version when built with `go install`, else the VCS
// revision `go build` stamps, else "dev".
func readVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			if len(s.Value) > 12 {
				return s.Value[:12]
			}
			return s.Value
		}
	}
	return "dev"
}
