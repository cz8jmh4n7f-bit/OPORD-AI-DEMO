// Command opord-api serves the OPORD AI governance HTTP API over the same
// orchestrator Service the CLI uses.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/anthropic"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/litellm"
	aimock "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/mock"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders/openai"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/api"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/config"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/events"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api exited", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL is not set (see .env.example)")
	}
	logger := newLogger(cfg)

	pool, err := db.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	aiReg := aiproviders.NewRegistry()
	aimock.Register(aiReg)
	openai.Register(aiReg)
	anthropic.Register(aiReg)
	litellm.Register(aiReg)

	resolver := creds.NewResolver(cfg.VaultAddr, cfg.VaultToken, cfg.VaultKVMount, logger)
	svc := orchestrator.New(db.New(pool), resolver, logger)
	svc.SetAIProviders(aiReg)

	// Connector bus: governance events to the audit log + Slack + SIEM.
	bus := events.FromConfig(events.SinkConfig{
		SlackWebhookURL: cfg.SlackWebhookURL,
		SIEMURL:         cfg.SIEMURL,
	}, logger)
	svc.SetEvents(bus)
	logger.Info("connectors enabled", "sinks", bus.Sinks())

	srv := api.NewServer(svc, logger)
	srv.SetAuth(svc.ResolveAPIKey, cfg.AuthEnabled)
	logger.Info("auth", "enabled", cfg.AuthEnabled)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Background loops share one cancellable context, stopped on shutdown.
	bgCtx, stopBg := context.WithCancel(context.Background())
	defer stopBg()

	// AI access-expiry reaper: revoke grants past their approved window so access
	// never outlives its expiry (a governance safety net).
	go runAIExpiryReaper(bgCtx, svc, logger)

	go func() {
		logger.Info("api listening", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen failed", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	stopBg()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}

// runAIExpiryReaper revokes AI access grants past their expiry on a fixed interval.
func runAIExpiryReaper(ctx context.Context, svc *orchestrator.Service, logger *slog.Logger) {
	const interval = 5 * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logger.Info("ai expiry reaper started", "interval", interval.String())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := svc.ReapExpiredAIInstances(ctx); err != nil {
				logger.Error("ai expiry reaper scan failed", "err", err)
			} else if n > 0 {
				logger.Info("ai expiry reaper revoked expired access", "count", n)
			}
		}
	}
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
