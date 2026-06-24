package engine

import (
	"context"
	"log/slog"
	"sync"

	"github.com/smarthome-bridge/internal/config"
	"github.com/smarthome-bridge/internal/mqtt"
)

// Engine is the processing pipeline: it receives MQTT messages, evaluates them
// against configured rules, and emits TriggerResult notifications.
type Engine struct {
	cfg       EngineConfig
	evaluator *Evaluator
	rules     []config.Rule

	inboundCh  <-chan mqtt.Message
	notifyCh   chan<- *TriggerResult
	workerWg   sync.WaitGroup
}

// EngineConfig tunes the engine's runtime behavior.
type EngineConfig struct {
	WorkerCount      int
	InboundQueueSize int
	NotifyQueueSize  int
}

// New creates a new Engine.
func New(cfg EngineConfig, inbound <-chan mqtt.Message, notify chan<- *TriggerResult, rules []config.Rule) *Engine {
	return &Engine{
		cfg:       cfg,
		evaluator: NewEvaluator(),
		rules:     rules,
		inboundCh: inbound,
		notifyCh:  notify,
	}
}

// Run starts the worker pool. Blocks until the inbound channel is closed
// and all workers have finished processing.
func (e *Engine) Run(ctx context.Context) {
	slog.Info("engine starting", "workers", e.cfg.WorkerCount)

	for i := 0; i < e.cfg.WorkerCount; i++ {
		e.workerWg.Add(1)
		go e.worker(ctx, i)
	}

	e.workerWg.Wait()

	// All workers done — close notify channel to signal the notifier pump.
	close(e.notifyCh)
	slog.Info("engine stopped")
}

// worker is a single goroutine that reads from inboundCh, evaluates rules,
// and pushes trigger results to notifyCh.
func (e *Engine) worker(ctx context.Context, id int) {
	defer e.workerWg.Done()
	defer e.recoverWorker(id)

	slog.Debug("engine worker started", "worker_id", id)

	for msg := range e.inboundCh {
		// Check context cancellation between messages.
		select {
		case <-ctx.Done():
			slog.Debug("engine worker stopping (ctx cancelled)", "worker_id", id)
			// Drain remaining messages from the channel (non-blocking reads).
			for {
				select {
				case msg, ok := <-e.inboundCh:
					if !ok {
						return
					}
					e.processMessage(msg)
				default:
					return
				}
			}
		default:
		}

		e.processMessage(msg)
	}

	slog.Debug("engine worker stopped (channel closed)", "worker_id", id)
}

// processMessage evaluates a single message against all enabled rules.
func (e *Engine) processMessage(msg mqtt.Message) {
	for i := range e.rules {
		rule := &e.rules[i]
		if !rule.IsEnabled() {
			continue
		}

		result := e.evaluator.Evaluate(msg.Topic, msg.Payload, rule)
		if result != nil {
			// Render template.
			parsed, err := e.evaluator.tmpl.GetOrParse(rule.Name, rule.MessageTemplate)
			if err != nil {
				slog.Error("template parse failed",
					"rule", rule.Name,
					"error", err,
				)
				continue
			}

			rendered, err := Render(parsed, TemplateData{
				RuleName:    result.RuleName,
				Description: result.Description,
				Device:      result.Device,
				Topic:       result.Topic,
				Fields:      result.Fields,
				Time:        result.Timestamp,
				Payload:     string(msg.Payload),
			})
			if err != nil {
				slog.Error("template render failed",
					"rule", rule.Name,
					"error", err,
				)
				continue
			}

			// Embed the rendered text into the result.
			// We use the Template field to carry the rendered message.
			result.Template = rendered

			// Push to notification channel (non-blocking with drop).
			select {
			case e.notifyCh <- result:
				slog.Debug("rule triggered",
					"rule", rule.Name,
					"device", result.Device,
					"topic", result.Topic,
				)
			default:
				slog.Warn("notify queue full, dropping trigger",
					"rule", rule.Name,
					"device", result.Device,
				)
			}
		}
	}
}

// recoverWorker catches panics in a worker goroutine and logs them.
func (e *Engine) recoverWorker(id int) {
	if r := recover(); r != nil {
		slog.Error("engine worker panicked",
			"worker_id", id,
			"panic", r,
		)
	}
}

// Evaluator returns the underlying Evaluator (for testing/diagnostics).
func (e *Engine) Evaluator() *Evaluator {
	return e.evaluator
}
