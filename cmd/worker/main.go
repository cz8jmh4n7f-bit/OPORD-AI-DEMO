// Command opord-worker runs the River job pool that executes OPORD's durable
// background work (provision/destroy of VMs and clusters). It shares the same
// orchestrator.Service the API and CLI use; jobs carry only ids, so the worker
// reloads everything from the database and survives restarts.
package main

import (
	"context"
	"errors"
	"log/slog"
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
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/config"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/events"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/ipam"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/jobs"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/orchestrator"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/aws"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/azure"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/gcp"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/proxmox"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers/vsphere"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

func main() {
	if err := run(); err != nil {
		slog.Error("worker exited", "err", err)
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
	vsphere.Register(reg, vsphere.Config{ModulesDir: cfg.ModulesDir, TofuBin: cfg.TofuBin, StateConnStr: cfg.DatabaseURL})
	proxmox.Register(reg, proxmox.Config{ModulesDir: cfg.ModulesDir, TofuBin: cfg.TofuBin, StateConnStr: cfg.DatabaseURL})
	aws.Register(reg, aws.Config{ModulesDir: cfg.ModulesDir, TofuBin: cfg.TofuBin, StateConnStr: cfg.DatabaseURL})
	azure.Register(reg, azure.Config{ModulesDir: cfg.ModulesDir, TofuBin: cfg.TofuBin, StateConnStr: cfg.DatabaseURL, Logger: logger})
	gcp.Register(reg, gcp.Config{ModulesDir: cfg.ModulesDir, TofuBin: cfg.TofuBin, StateConnStr: cfg.DatabaseURL, Logger: logger})
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
	// The worker runs provisioning, so it emits the ready/failed/destroyed events.
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

	// CIDR IPAM (Vault-backed) for the account factory's secure-VPC layer (optional).
	if cfg.VaultAddr != "" && cfg.VaultToken != "" {
		if cidrPool, err := ipam.NewVaultPool(cfg.VaultAddr, cfg.VaultToken, "opord-vpc-cidr-pools", "aws-vpc-cidr-pools", logger); err != nil {
			logger.Warn("ipam init failed; account secure-VPC needs an explicit vpc_cidr", "err", err)
		} else {
			svc.SetAllocator(cidrPool)
			logger.Info("cidr ipam enabled", "mount", "opord-vpc-cidr-pools")
		}
	}

	// Ensure River's schema exists, then start the worker pool.
	ctx := context.Background()
	if err := jobs.Migrate(ctx, pool); err != nil {
		return err
	}
	client, err := jobs.NewWorkerClient(pool, svc, 10)
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	logger.Info("worker started; processing jobs")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down worker")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return client.Stop(shutdownCtx)
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
