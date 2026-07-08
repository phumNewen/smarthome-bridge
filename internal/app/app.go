package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
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

	// Send startup notification.
	go a.sendStartup(ctx, tgClient)

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

// sendStartup sends a startup notification.
// Uses ADMIN_CHAT_ID env var if set, otherwise collects chat IDs from enabled rules.
func (a *App) sendStartup(ctx context.Context, tg *notifier.TelegramClient) {
	chats := adminChatID()
	fromEnv := len(chats) > 0
	if len(chats) == 0 {
		chats = collectChatIDs(a.cfg.Rules)
	}
	if len(chats) == 0 {
		slog.Warn("startup notification skipped: no chat IDs configured")
		return
	}
	slog.Info("sending startup notification",
		"chats", chats,
		"from_admin_env", fromEnv,
	)
	msg := fmt.Sprintf("✅ smarthome-bridge started\n%s", time.Now().Format("2006-01-02 15:04:05"))
	if err := tg.SendMessage(ctx, chats, msg); err != nil {
		slog.Warn("startup notification failed", "error", err)
	} else {
		slog.Info("startup notification sent")
	}
}

// adminChatID reads ADMIN_CHAT_ID from env (comma-separated list).
func adminChatID() []int64 {
	raw := os.Getenv("ADMIN_CHAT_ID")
	if raw == "" {
		return nil
	}
	var chats []int64
	for _, s := range splitAndTrim(raw, ",") {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			chats = append(chats, id)
		}
	}
	return chats
}

// splitAndTrim splits a string by separator and trims each part.
func splitAndTrim(s, sep string) []string {
	var parts []string
	for _, p := range strings.Split(s, sep) {
		t := strings.TrimSpace(p)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return parts
}

// collectChatIDs returns unique chat IDs from enabled rules.
func collectChatIDs(rules []config.Rule) []int64 {
	seen := make(map[int64]bool)
	var chats []int64
	for _, r := range rules {
		if !r.IsEnabled() {
			continue
		}
		for _, id := range r.ChatIDs {
			if !seen[id] {
				seen[id] = true
				chats = append(chats, id)
			}
		}
	}
	return chats
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
