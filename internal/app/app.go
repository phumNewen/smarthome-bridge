package app

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/smarthome-bridge/internal/config"
	"github.com/smarthome-bridge/internal/engine"
	"github.com/smarthome-bridge/internal/mqtt"
	"github.com/smarthome-bridge/internal/notifier"
)

// App is the top-level orchestrator that wires all components together.
type App struct {
	cfg *config.Config
}

// New creates a new App instance.
func New(cfg *config.Config) *App {
	return &App{cfg: cfg}
}

// Run starts all components and blocks until the context is cancelled.
// It implements graceful shutdown: drains in-flight messages before returning.
func (a *App) Run(ctx context.Context) error {
	slog.Info("application starting")

	// Create channels.
	inboundCh := make(chan mqtt.Message, a.cfg.Engine.InboundQueueSize)
	notifyCh := make(chan *engine.TriggerResult, a.cfg.Engine.NotifyQueueSize)

	// Create components.
	mqttSub := mqtt.NewSubscriber(
		mqtt.Config{
			Broker:            a.cfg.MQTT.Broker,
			ClientID:          a.cfg.MQTT.ClientID,
			Username:          a.cfg.MQTT.Username,
			Password:          a.cfg.MQTT.Password,
			KeepAliveSec:      a.cfg.MQTT.KeepAliveSec,
			ConnectTimeoutSec: a.cfg.MQTT.ConnectTimeoutSec,
			PingTimeoutSec:    a.cfg.MQTT.PingTimeoutSec,
			Subscriptions:     toMQTTSubscriptions(a.cfg.MQTT.Subscriptions),
		},
		inboundCh,
	)

	eng := engine.New(
		engine.EngineConfig{
			WorkerCount:      a.cfg.Engine.WorkerCount,
			InboundQueueSize: a.cfg.Engine.InboundQueueSize,
			NotifyQueueSize:  a.cfg.Engine.NotifyQueueSize,
		},
		inboundCh,
		notifyCh,
		a.cfg.Rules,
	)

	tgClient := notifier.NewTelegramClient(
		notifier.TelegramConfig{
			APIBaseURL:     a.cfg.Telegram.APIBaseURL,
			BotToken:       a.cfg.Telegram.BotToken,
			RetryMax:       a.cfg.Telegram.RetryMax,
			RetryBackoffMs: a.cfg.Telegram.RetryBackoffMs,
		},
	)

	// Pre-parse all templates.
	eng.Evaluator().TemplateCache()

	// Connect to MQTT broker.
	if err := mqttSub.Connect(ctx); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}
	slog.Info("MQTT connected successfully")

	// Start engine worker pool.
	var engineWg sync.WaitGroup
	engineWg.Add(1)
	go func() {
		defer engineWg.Done()
		eng.Run(ctx)
	}()

	// Start notification pump.
	var notifyWg sync.WaitGroup
	notifyWg.Add(1)
	go func() {
		defer notifyWg.Done()
		a.notifyPump(ctx, notifyCh, tgClient)
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	slog.Info("shutdown signal received, draining...")

	// Graceful shutdown sequence.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Disconnect MQTT (stops new messages from arriving).
	mqttSub.Disconnect(250)

	// Step 2: Close inbound channel so workers start draining.
	close(inboundCh)

	// Step 3: Wait for engine workers to finish.
	engineWg.Wait()
	// notifyCh is closed by engine.Run() after all workers exit.

	// Step 4: Wait for notify pump to drain remaining notifications.
	notifyWg.Wait()

	// Step 5: Stop background goroutines.
	eng.Evaluator().Cooldown().Stop()

	if shutdownCtx.Err() != nil {
		slog.Warn("graceful shutdown timed out, some messages may be lost")
	}

	slog.Info("application stopped")
	return nil
}

// notifyPump reads trigger results from the channel and sends them via Telegram.
func (a *App) notifyPump(ctx context.Context, ch <-chan *engine.TriggerResult, tg *notifier.TelegramClient) {
	slog.Info("notify pump started")

	for result := range ch {
		if err := tg.Send(ctx, notifier.Notification{
			ChatIDs:   result.ChatIDs,
			Text:      result.Template, // Already rendered by the engine.
			ParseMode: "HTML",
		}); err != nil {
			slog.Error("notification send failed",
				"rule", result.RuleName,
				"device", result.Device,
				"error", err,
			)
		}
	}

	slog.Info("notify pump stopped")
}

// toMQTTSubscriptions converts config subscriptions to mqtt package subscriptions.
func toMQTTSubscriptions(subs []config.Subscription) []mqtt.SubscriptionConfig {
	result := make([]mqtt.SubscriptionConfig, len(subs))
	for i, s := range subs {
		result[i] = mqtt.SubscriptionConfig{
			Topic: s.Topic,
			QoS:   s.QoS,
		}
	}
	return result
}
