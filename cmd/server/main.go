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
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"

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
	"github.com/thomas-elliott/damson/internal/services/uploads"
	damsonweb "github.com/thomas-elliott/damson/internal/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "err", err)
		os.Exit(1)
	}
}

// run starts the server and blocks until a signal stops it. A startup error
// is returned for main to log, so deferred cleanup still runs.
func run() error {
	logger, err := newLogger(os.Stdout)
	slog.SetDefault(logger)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.GinMode != "" {
		gin.SetMode(cfg.GinMode)
	}
	if err := checkDataDirs(cfg); err != nil {
		return fmt.Errorf("data directory: %w", err)
	}

	gormDB, err := db.Open(cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("db open: %w", err)
	}
	// Deferred first, so it runs last: after the queue and the background
	// goroutines that use the pool have stopped.
	defer closeDB(gormDB)

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	if err := db.Migrate(startupCtx, gormDB); err != nil {
		return fmt.Errorf("migrate: %w", err)
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
			return fmt.Errorf("oidc init: %w", err)
		}
		authenticator = oa
		slog.Info("auth mode", "mode", "oidc", "issuer", cfg.OIDCIssuerURL)

	case config.AuthModeLocal:
		signingKey := cfg.JWTSigningKey
		if len(signingKey) == 0 {
			k, err := auth.LoadOrCreateJWTKey(cfg.JWTKeyFile)
			if err != nil {
				return fmt.Errorf("jwt key: %w", err)
			}
			signingKey = k
		}
		la, err := auth.NewLocalAuthenticatorWithIssuer(gormDB, signingKey, cfg.BaseURL)
		if err != nil {
			return fmt.Errorf("local auth init: %w", err)
		}
		if err := auth.SeedLocalAdmin(startupCtx, gormDB, cfg.LocalAdminEmail, cfg.LocalAdminPassword); err != nil {
			return fmt.Errorf("seed local admin: %w", err)
		}
		authenticator = la
		localAuth = la
		slog.Info("auth mode", "mode", "local")
	}

	store, err := storage.New(startupCtx, cfg)
	if err != nil {
		return fmt.Errorf("storage init: %w", err)
	}
	slog.Info("storage mode", "mode", cfg.StorageMode)

	previewBroadcaster := preview.NewBroadcaster()
	broadcaster := imports.NewBroadcaster()
	metaRegistry := metadata.Default(metadata.ClientOptions{StoreLookupInterval: cfg.StoreLookupInterval})

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
		return fmt.Errorf("queue init: %w", err)
	}
	importEnqueuer := jobQueue.Imports()
	previewEnqueuer := jobQueue.Previews()

	// Reconcile: re-enqueue any non-terminal domain row without a live job.
	// Idempotent (River dedupes by args), so it's safe every boot and covers the
	// one-time cutover of rows that were in flight before River existed. Previews
	// first — ResumeInFlight resets processing→queued for files a worker died on.
	// Files an earlier build could not render (the tier-2 image formats) are
	// queued first, so the reconcile below enqueues them with the rest.
	if queued, err := preview.QueueNewlySupported(startupCtx, gormDB); err != nil {
		slog.Warn("preview backfill failed", "err", err)
	} else if queued > 0 {
		slog.Info("preview backfill", "queued", queued)
	}
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

	// Each background goroutine logs its own failure and returns nil: a plain
	// Group, not WithContext, so one failing does not stop the others.
	bgCtx, cancelBackground := context.WithCancel(context.Background())
	var background errgroup.Group
	defer func() {
		cancelBackground()
		waitBackground(&background, shutdownTimeout)
	}()
	// Off the startup path: each refetch waits on the storefront's rate limit.
	background.Go(func() error {
		if err := imports.RepairGallery(bgCtx, gormDB, store, metaRegistry); err != nil {
			slog.Warn("gallery repair failed", "err", err)
		}
		return nil
	})
	// Off the startup path: it reads blobs.
	background.Go(func() error {
		if covered, err := imports.BackfillCovers(bgCtx, gormDB, store); err != nil {
			slog.Warn("cover backfill failed", "err", err)
		} else if covered > 0 {
			slog.Info("cover backfill", "covered", covered)
		}
		return nil
	})
	purger := &audit.Purger{DB: gormDB, Interval: cfg.AuditPurgeInterval}
	background.Go(func() error { purger.Run(bgCtx); return nil })
	janitor := &uploads.Janitor{DB: gormDB, Storage: store, Interval: cfg.UploadJanitorInterval}
	background.Go(func() error { janitor.Run(bgCtx); return nil })

	r, err := newRouter(oidcCSPOrigin(cfg), cfg.TrustedProxies)
	if err != nil {
		return fmt.Errorf("DAMSON_TRUSTED_PROXIES: %w", err)
	}
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
		StoreLookups:       jobQueue.StoreLookups(),
		OrphanMinAge:       cfg.OrphanMinAge,
		// Echoed to the SPA on /api/auth/mode in OIDC mode (empty in local).
		OIDCAuthority: cfg.OIDCIssuerURL,
		OIDCClientID:  cfg.OIDCClientID,
	})
	api.AddRoutes(apiGroup, auth.Middleware(authenticator, auth.NewTokenAuthenticator(gormDB)), h)

	if err := damsonweb.Mount(r); err != nil {
		return fmt.Errorf("mount spa: %w", err)
	}

	// The queue starts only now: store lookup jobs run through the handler,
	// which is built after the queue it enqueues on. Start with a long-lived
	// context: cancelling Start's context soft-stops the client, so shutdown is
	// driven by Stop below, not by startupCtx expiring.
	jobQueue.SetStoreLookupRunner(h)
	if err := jobQueue.Start(context.Background()); err != nil {
		return fmt.Errorf("queue start: %w", err)
	}
	slog.Info("job queue started", "import_workers", cfg.ImportQueueWorkers, "preview_workers", cfg.PreviewQueueWorkers)
	if resumed, err := h.ResumeStoreLookups(startupCtx); err != nil {
		slog.Warn("store lookup reconcile failed", "err", err)
	} else if resumed > 0 {
		slog.Info("store lookup reconcile", "enqueued", resumed)
	}

	srv, stopStreams := newHTTPServer(cfg.ListenAddr, r)

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "addr", cfg.ListenAddr)
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// A listen failure still stops the queue, so no worker outlives the pool.
	var listenErr error
	select {
	case sig := <-stop:
		slog.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			listenErr = fmt.Errorf("listen: %w", err)
		}
	}

	cancelBackground()
	shutdown(srv, jobQueue, shutdownTimeout, stopStreams)
	return listenErr
}

// shutdownTimeout bounds each step of shutdown.
const shutdownTimeout = 10 * time.Second

// newHTTPServer returns the server and the func that ends its SSE streams.
// Every request context carries the streams' stopping signal (see
// api.WithServerStopping) without being cancelled by it, so a stream ends
// at shutdown while other in-flight requests finish.
func newHTTPServer(addr string, h http.Handler) (*http.Server, context.CancelFunc) {
	stopping, stopStreams := context.WithCancel(context.Background())
	base := api.WithServerStopping(context.Background(), stopping)
	// Timeouts:
	//   ReadHeaderTimeout — defends against slow-loris on the request line.
	//   ReadTimeout       — covers the body too; uploads can be large so this
	//                       is generous. Per-handler control happens via
	//                       request context cancellation.
	//   WriteTimeout      — bounds long-running responses (file streams).
	//   IdleTimeout       — keeps keep-alive connections from accruing.
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		BaseContext:       func(net.Listener) context.Context { return base },
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Minute,
		WriteTimeout:      30 * time.Minute,
		IdleTimeout:       2 * time.Minute,
	}, stopStreams
}

// waitBackground waits for the background goroutines, giving up after
// timeout so a goroutine that ignores its context cannot hold the process.
func waitBackground(g *errgroup.Group, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		_ = g.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		slog.Warn("background goroutines still running at shutdown", "timeout", timeout)
	}
}

func closeDB(gormDB *gorm.DB) {
	sqlDB, err := gormDB.DB()
	if err == nil {
		err = sqlDB.Close()
	}
	if err != nil {
		slog.Warn("db close", "err", err)
	}
}

// shutdown stops HTTP first, ending the SSE streams through stopStreams so
// they don't hold it to the timeout, and lets in-flight requests finish
// enqueueing their jobs. Then it drains the queue. Each step gets its own timeout; work still running
// when one expires is abandoned, and the next boot's reconcile picks up
// whatever remained non-terminal.
func shutdown(srv *http.Server, jobs interface{ Stop(context.Context) error }, timeout time.Duration, stopStreams func()) {
	srv.RegisterOnShutdown(stopStreams)
	httpCtx, cancelHTTP := context.WithTimeout(context.Background(), timeout)
	defer cancelHTTP()
	if err := srv.Shutdown(httpCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
	queueCtx, cancelQueue := context.WithTimeout(context.Background(), timeout)
	defer cancelQueue()
	if err := jobs.Stop(queueCtx); err != nil {
		slog.Warn("queue stop", "err", err)
	}
}

// newRouter builds the engine with the global middleware every request runs
// through, in order. Tests use it to reach a handler the way production does.
//
// trustedProxies lists the proxies (IPs or CIDRs) whose X-Forwarded-For is
// believed. Empty trusts none, so ClientIP — which the login and bootstrap
// throttles key on — is the connection's own address and not caller-chosen.
func newRouter(cspOrigin string, trustedProxies []string) (*gin.Engine, error) {
	r := gin.New()
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		return nil, err
	}
	r.Use(requestID())
	r.Use(requestLogger())
	r.Use(gin.CustomRecovery(recoveryHandler))
	r.Use(securityHeaders(cspOrigin))
	r.Use(jsonBodyCap(maxJSONBodyBytes, "/api"+api.UploadChunkRoute))
	return r, nil
}

// maxJSONBodyBytes caps the body Gin will read on JSON-shaped requests.
// Multipart uploads (POST /api/imports) and upload chunks are exempt — they
// handle their own streaming and caps, see internal/handlers/api. 1 MiB is well above
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
// reads. 413 surfaces if the limit trips. ownCap names routes whose handler
// caps the body itself, at a size above limit (the upload chunk PUT).
func jsonBodyCap(limit int64, ownCap ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ct := c.Request.Header.Get("Content-Type")
		if c.Request.Body != nil && !strings.HasPrefix(ct, "multipart/") && !slices.Contains(ownCap, c.FullPath()) {
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

// newLogger builds the production logger from DAMSON_LOG_LEVEL and
// DAMSON_LOG_FORMAT. An unknown value is an error, returned alongside a JSON
// logger at info so the caller can still log it.
func newLogger(w io.Writer) (*slog.Logger, error) {
	var errs []error
	level := slog.LevelInfo
	switch v := os.Getenv("DAMSON_LOG_LEVEL"); v {
	case "", "info":
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		errs = append(errs, fmt.Errorf("config: DAMSON_LOG_LEVEL must be debug, info, warn or error, got %q", v))
	}
	opts := &slog.HandlerOptions{Level: level, AddSource: true}
	var h slog.Handler = slog.NewJSONHandler(w, opts)
	switch v := os.Getenv("DAMSON_LOG_FORMAT"); v {
	case "", "json":
	case "text":
		h = slog.NewTextHandler(w, opts)
	default:
		errs = append(errs, fmt.Errorf("config: DAMSON_LOG_FORMAT must be json or text, got %q", v))
	}
	return slog.New(h).With("service", "damson", "version", readVersion()), errors.Join(errs...)
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
