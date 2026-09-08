// Command dmserver runs the reference Windows MDM server.
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

	"github.com/deploymenttheory/go-microsoft-dm/server/internal/app"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(logger); err != nil {
		logger.Error("dmserver", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := app.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg, app.Options{Log: func(method, path string, status int) {
		logger.Info("request", "method", method, "path", path, "status", status)
	}})
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	if cfg.Memory {
		logger.Warn("using an ephemeral in-memory store; state is lost on exit")
	}
	if cfg.CACert == "" {
		logger.Warn("using an ephemeral self-signed enrollment CA; set DM_CA_CERT and DM_CA_KEY to persist it")
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           a.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Listen, "tls", cfg.TLSCert != "", "base_url", cfg.BaseURL)
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			errc <- srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			errc <- srv.ListenAndServe()
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
