package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/smarthome-bridge/internal/app"
	"github.com/smarthome-bridge/internal/config"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to YAML configuration file")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	// Set up structured JSON logging to stderr.
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	})))

	slog.Info("smarthome-bridge starting", "config", *configPath)

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("configuration loaded",
		"mqtt_broker", cfg.MQTT.Broker,
		"rules_count", len(cfg.Rules),
		"subscriptions_count", len(cfg.MQTT.Subscriptions),
	)

	// Count enabled rules.
	enabled := 0
	for _, r := range cfg.Rules {
		if r.IsEnabled() {
			enabled++
		}
	}
	if enabled == 0 {
		slog.Warn("no enabled rules found — the service will connect to MQTT but never notify")
	}
	slog.Info(fmt.Sprintf("rules: %d total, %d enabled", len(cfg.Rules), enabled))

	// Trap OS signals for graceful shutdown.
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP,
	)
	defer cancel()

	// Create and run the application.
	application := app.New(cfg)
	if err := application.Run(ctx); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}

	slog.Info("shutdown complete")
}
