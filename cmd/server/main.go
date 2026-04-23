// Damson server entry point. Wires config, DB (with migrations), OIDC or
// local authentication, HTTP handlers, and the embedded SPA into a single
// process.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/db"
	"github.com/thomas-elliott/damson/internal/handlers/api"
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
			IssuerURL:       cfg.OIDCIssuerURL,
			ClientID:        cfg.OIDCClientID,
			Audience:        cfg.OIDCAudience,
			BootstrapAdmins: cfg.BootstrapAdmins,
		})
		if err != nil {
			slog.Error("oidc init", "err", err)
			os.Exit(1)
		}
		authenticator = oa
		slog.Info("auth mode", "mode", "oidc", "issuer", cfg.OIDCIssuerURL)

	case config.AuthModeLocal:
		la, err := auth.NewLocalAuthenticator(gormDB, cfg.JWTSigningKey)
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

	r := gin.New()
	r.Use(gin.Recovery())

	apiGroup := r.Group("/api")
	h := api.New(api.Deps{
		DB:       gormDB,
		AuthMode: cfg.AuthMode,
		Local:    localAuth,
		Storage:  store,
	})
	api.AddRoutes(apiGroup, auth.Middleware(authenticator), h)

	if err := damsonweb.Mount(r); err != nil {
		slog.Error("mount spa", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           r,
		ReadHeaderTimeout: 30 * time.Second,
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
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "err", err)
	}
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
	opts := &slog.HandlerOptions{Level: level}
	if os.Getenv("DAMSON_LOG_FORMAT") == "text" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
