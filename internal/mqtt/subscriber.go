package mqtt

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Message is the normalized representation of a received MQTT message.
type Message struct {
	Topic     string
	Payload   []byte
	QoS       byte
	Timestamp time.Time
}

// Subscriber connects to an MQTT broker and forwards messages to a channel.
type Subscriber struct {
	cfg       Config
	client    mqtt.Client
	msgCh     chan<- Message
	mu        sync.Mutex
	connected bool
}

// Config holds the parameters needed by the Subscriber.
type Config struct {
	Broker            string
	ClientID          string
	Username          string
	Password          string
	KeepAliveSec      int
	ConnectTimeoutSec int
	PingTimeoutSec    int
	Subscriptions     []SubscriptionConfig
}

// SubscriptionConfig describes a single topic subscription.
type SubscriptionConfig struct {
	Topic string
	QoS   byte
}

// NewSubscriber creates a Subscriber. It does not connect to the broker yet.
func NewSubscriber(cfg Config, msgCh chan<- Message) *Subscriber {
	return &Subscriber{
		cfg:   cfg,
		msgCh: msgCh,
	}
}

// Connect establishes the MQTT connection and subscribes to topics.
// It blocks until the initial connection is established or the context is cancelled.
func (s *Subscriber) Connect(ctx context.Context) error {
	opts := mqtt.NewClientOptions().
		AddBroker(s.cfg.Broker).
		SetClientID(s.cfg.ClientID).
		SetKeepAlive(time.Duration(s.cfg.KeepAliveSec) * time.Second).
		SetConnectTimeout(time.Duration(s.cfg.ConnectTimeoutSec) * time.Second).
		SetPingTimeout(time.Duration(s.cfg.PingTimeoutSec) * time.Second).
		SetAutoReconnect(true).
		SetCleanSession(true).
		SetOrderMatters(false).
		SetOnConnectHandler(func(c mqtt.Client) {
			slog.Info("MQTT connected", "broker", s.cfg.Broker)
			s.mu.Lock()
			s.connected = true
			s.mu.Unlock()

			// Re-subscribe on reconnect.
			for _, sub := range s.cfg.Subscriptions {
				token := c.Subscribe(sub.Topic, sub.QoS, nil)
				token.Wait()
				if err := token.Error(); err != nil {
					slog.Error("MQTT re-subscribe failed", "topic", sub.Topic, "error", err)
				} else {
					slog.Info("MQTT re-subscribed", "topic", sub.Topic, "qos", sub.QoS)
				}
			}
		}).
		SetConnectionLostHandler(func(c mqtt.Client, err error) {
			slog.Warn("MQTT connection lost", "error", err)
			s.mu.Lock()
			s.connected = false
			s.mu.Unlock()
		}).
		SetDefaultPublishHandler(func(c mqtt.Client, msg mqtt.Message) {
			select {
			case s.msgCh <- Message{
				Topic:     msg.Topic(),
				Payload:   msg.Payload(),
				QoS:       msg.Qos(),
				Timestamp: time.Now(),
			}:
			case <-time.After(5 * time.Second):
				slog.Error("MQTT inbound queue stuck, dropping message after timeout",
					"topic", msg.Topic(),
					"queue_len", len(s.msgCh),
					"queue_cap", cap(s.msgCh),
				)
			}
		})

	if s.cfg.Username != "" {
		opts.SetUsername(s.cfg.Username)
	}
	if s.cfg.Password != "" {
		opts.SetPassword(s.cfg.Password)
	}

	s.client = mqtt.NewClient(opts)
	token := s.client.Connect()
	if !token.WaitTimeout(time.Duration(s.cfg.ConnectTimeoutSec) * time.Second) {
		return fmt.Errorf("MQTT connect timed out after %ds", s.cfg.ConnectTimeoutSec)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT connect failed: %w", err)
	}

	// Subscribe to topics.
	for _, sub := range s.cfg.Subscriptions {
		token := s.client.Subscribe(sub.Topic, sub.QoS, nil)
		token.Wait()
		if err := token.Error(); err != nil {
			return fmt.Errorf("subscribe to %q failed: %w", sub.Topic, err)
		}
		slog.Info("MQTT subscribed", "topic", sub.Topic, "qos", sub.QoS)
	}

	s.mu.Lock()
	s.connected = true
	s.mu.Unlock()

	return nil
}

// IsConnected returns the current connection state.
func (s *Subscriber) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

// Disconnect gracefully closes the MQTT connection.
func (s *Subscriber) Disconnect(quiesceMs uint) {
	slog.Info("MQTT disconnecting")
	s.client.Disconnect(quiesceMs)
	s.mu.Lock()
	s.connected = false
	s.mu.Unlock()
}
