// Command opord-api serves the OPORD HTTP API over the same orchestrator
// Service the CLI uses.
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
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/azure"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/config"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/events"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/glpi"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/ipam"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/jobs"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/aws"
	azureprov "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/azure"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/gcp"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/proxmox"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/vsphere"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/reconciler"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
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

	reg := providers.NewRegistry()
	vsphere.Register(reg, vsphere.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	proxmox.Register(reg, proxmox.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	aws.Register(reg, aws.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
	})
	azureprov.Register(reg, azureprov.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
		Logger:       logger,
	})
	gcp.Register(reg, gcp.Config{
		ModulesDir:   cfg.ModulesDir,
		TofuBin:      cfg.TofuBin,
		StateConnStr: cfg.DatabaseURL,
		Logger:       logger,
	})
	aiReg := aiproviders.NewRegistry()
	aimock.Register(aiReg)
	openai.Register(aiReg)
	anthropic.Register(aiReg)
	litellm.Register(aiReg)

	resolver := creds.NewResolver(cfg.VaultAddr, cfg.VaultToken, cfg.VaultKVMount, logger)
	if pass := creds.StateEncryptionPassphrase(context.Background(), resolver); pass != "" {
		tofu.SetStateEncryptionPassphrase(pass)
		logger.Info("tofu state encryption enabled")
	}
	svc := orchestrator.New(db.New(pool), reg, resolver, logger, orchestrator.BootstrapConfig{
		AnsibleBin:    cfg.AnsibleBin,
		AnsibleDir:    cfg.AnsibleDir,
		SSHPrivateKey: cfg.SSHPrivateKey,
		ArtifactsDir:  cfg.ArtifactsDir,
	})
	svc.SetAIProviders(aiReg)

	// Hand long-running work to River (durable, survives restarts) when the
	// queue is reachable; the opord-worker process executes it. If River setup
	// fails, the Service falls back to in-process goroutines.
	// Connector bus: lifecycle events to audit log + Slack + SIEM + GLPI CMDB.
	bus := events.FromConfig(events.SinkConfig{
		SlackWebhookURL: cfg.SlackWebhookURL,
		SIEMURL:         cfg.SIEMURL,
		GLPIURL:         cfg.GLPIURL,
		GLPIAppToken:    cfg.GLPIAppToken,
		GLPIUserToken:   cfg.GLPIUserToken,
		GLPIItemType:    cfg.GLPIItemType,
	}, logger)
	svc.SetEvents(bus)
	logger.Info("connectors enabled", "sinks", bus.Sinks())

	// GLPI ticketer for the request/approval workflow (optional).
	if cfg.GLPIURL != "" && cfg.GLPIAppToken != "" && cfg.GLPIUserToken != "" {
		svc.SetTicketer(glpi.New(cfg.GLPIURL, cfg.GLPIAppToken, cfg.GLPIUserToken))
		logger.Info("glpi ticketer enabled", "url", cfg.GLPIURL)
	}

	// CIDR IPAM (Vault-backed) for the account factory's secure-VPC layer (optional).
	if cfg.VaultAddr != "" && cfg.VaultToken != "" {
		if pool, err := ipam.NewVaultPool(cfg.VaultAddr, cfg.VaultToken, "opord-vpc-cidr-pools", "aws-vpc-cidr-pools", logger); err != nil {
			logger.Warn("ipam init failed; account secure-VPC needs an explicit vpc_cidr", "err", err)
		} else {
			svc.SetAllocator(pool)
			logger.Info("cidr ipam enabled", "mount", "opord-vpc-cidr-pools")
		}
	}

	// Microsoft Graph (Entra) client for SAML access automation: env override,
	// else Vault KV at opord/azure/graph. Optional.
	azCfg := azure.Config{TenantID: cfg.AzureTenantID, ClientID: cfg.AzureClientID, ClientSecret: cfg.AzureClientSecret}
	if azCfg.TenantID == "" || azCfg.ClientID == "" || azCfg.ClientSecret == "" {
		if sec, err := resolver.ReadSecret(context.Background(), "opord/azure/graph"); err == nil && sec != nil {
			if azCfg.TenantID == "" {
				azCfg.TenantID = sec["tenant_id"]
			}
			if azCfg.ClientID == "" {
				azCfg.ClientID = sec["client_id"]
			}
			if azCfg.ClientSecret == "" {
				azCfg.ClientSecret = sec["client_secret"]
			}
		}
	}
	if azCfg.TenantID != "" && azCfg.ClientID != "" && azCfg.ClientSecret != "" {
		svc.SetEntra(azure.New(azCfg, logger))
		logger.Info("entra graph client enabled")
	}

	srv := api.NewServer(svc, logger)
	srv.SetAuth(svc.ResolveAPIKey, cfg.AuthEnabled)
	logger.Info("auth", "enabled", cfg.AuthEnabled)
	if err := jobs.Migrate(context.Background(), pool); err != nil {
		logger.Warn("river migrate failed; using in-process jobs", "err", err)
	} else if enq, err := jobs.NewEnqueuer(pool); err != nil {
		logger.Warn("river enqueuer init failed; using in-process jobs", "err", err)
	} else {
		svc.SetEnqueuer(enq)
		srv.SetJobLister(enq)
		logger.Info("river job queue enabled")
	}

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Background loops share one cancellable context, stopped on shutdown.
	bgCtx, stopBg := context.WithCancel(context.Background())
	defer stopBg()

	// TTL reaper: auto-destroys VMs whose ttl_hours has elapsed so forgotten
	// test instances don't linger and accrue cloud cost.
	go runReaper(bgCtx, svc, logger)

	// Drift reconciler: periodically `tofu plan`s ready clusters; flags drift as
	// degraded. Interval from OPORD_RECONCILE_INTERVAL ("0" disables).
	reconcileEvery, err := time.ParseDuration(cfg.ReconcileInterval)
	if err != nil {
		logger.Warn("invalid OPORD_RECONCILE_INTERVAL; disabling reconciler", "value", cfg.ReconcileInterval, "err", err)
		reconcileEvery = 0
	}
	scan := func(ctx context.Context) (int, int, int, error) {
		rep, err := svc.ReconcileClusters(ctx)
		return rep.Checked, rep.Drifted, rep.Errored, err
	}
	go reconciler.New("drift reconciler", scan, reconcileEvery, logger).Run(bgCtx)

	// Provider health checker: periodically probes each provider's backend
	// (reachability + creds) and persists the result, so provider health is
	// monitorable via GET /providers. Interval from OPORD_PROVIDER_CHECK_INTERVAL
	// ("0" / default disables - it makes outbound auth attempts, so it's opt-in).
	checkEvery, err := time.ParseDuration(cfg.ProviderCheckInterval)
	if err != nil {
		logger.Warn("invalid OPORD_PROVIDER_CHECK_INTERVAL; disabling provider health checks", "value", cfg.ProviderCheckInterval, "err", err)
		checkEvery = 0
	}
	go reconciler.New("provider health checker", svc.CheckAllProviders, checkEvery, logger).Run(bgCtx)

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

// runReaper scans for TTL-expired VMs on a fixed interval until ctx is done.
func runReaper(ctx context.Context, svc *orchestrator.Service, logger *slog.Logger) {
	const interval = 5 * time.Minute
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logger.Info("ttl reaper started", "interval", interval.String())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := svc.ReapExpiredVMs(ctx)
			if err != nil {
				logger.Error("ttl reaper scan failed", "err", err)
			} else if n > 0 {
				logger.Info("ttl reaper destroyed expired vms", "count", n)
			}
			// AI access expiry: revoke grants past their approved window (governance
			// safety net - access must not outlive its expiry).
			if ai, err := svc.ReapExpiredAIInstances(ctx); err != nil {
				logger.Error("ai expiry reaper scan failed", "err", err)
			} else if ai > 0 {
				logger.Info("ai expiry reaper revoked expired access", "count", ai)
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
