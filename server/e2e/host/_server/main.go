//go:build host

// Command host runs the reference app with private wire capture and optional experiments.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/server/e2e/host"
	"github.com/deploymenttheory/go-microsoft-dm/server/internal/app"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := app.Load()
	if err != nil {
		return err
	}
	if cfg.Listen != "127.0.0.1:8443" || cfg.BaseURL != "https://localhost:8443" {
		return fmt.Errorf("capture harness requires the localhost test endpoint")
	}
	// The conformance probes deliberately ask whether unknown CSP nodes exist.
	cfg.DisableSchemaValidation = true
	a, err := app.New(context.Background(), cfg, app.Options{})
	if err != nil {
		return err
	}
	defer a.Close()
	dir := os.Getenv("DM_CAPTURE_DIR")
	if dir == "" {
		return fmt.Errorf("DM_CAPTURE_DIR is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	recorder := &host.Recorder{Next: a.Handler, Directory: dir, ExperimentFile: os.Getenv("DM_EXPERIMENT_FILE")}
	srv := &http.Server{Addr: cfg.Listen, Handler: recorder, ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
}
