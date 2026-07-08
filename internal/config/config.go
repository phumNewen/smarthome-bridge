package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level application configuration.
type Config struct {
	MQTT     MQTTConfig     `yaml:"mqtt"`
	Telegram TelegramConfig `yaml:"telegram"`
	Engine   EngineConfig   `yaml:"engine"`
	Rules    []Rule         `yaml:"rules"`
}

// MQTTConfig holds broker connection parameters.
type MQTTConfig struct {
	Broker            string         `yaml:"broker"`
	ClientID          string         `yaml:"client_id"`
	Username          string         `yaml:"username"`
	Password          string         `yaml:"password"`
	KeepAliveSec      int            `yaml:"keep_alive_sec"`
	ConnectTimeoutSec int            `yaml:"connect_timeout_sec"`
	PingTimeoutSec    int            `yaml:"ping_timeout_sec"`
	Subscriptions     []Subscription `yaml:"subscriptions"`
}

// Subscription defines an MQTT topic subscription.
type Subscription struct {
	Topic string `yaml:"topic"`
	QoS   byte   `yaml:"qos"`
}

// TelegramConfig holds bot credentials and HTTP client settings.
type TelegramConfig struct {
	BotToken       string `yaml:"bot_token"`
	APIBaseURL     string `yaml:"api_base_url"`
	RetryMax       int    `yaml:"retry_max"`
	RetryBackoffMs []int  `yaml:"retry_backoff_ms"`
}

// EngineConfig tunes the processing pipeline.
type EngineConfig struct {
	WorkerCount      int `yaml:"worker_count"`
	InboundQueueSize int `yaml:"inbound_queue_size"`
	NotifyQueueSize  int `yaml:"notify_queue_size"`
}

// Rule defines a notification rule: when conditions are met, send a Telegram message.
type Rule struct {
	Name              string      `yaml:"name"`
	Description       string      `yaml:"description"`
	Enabled           *bool       `yaml:"enabled"` // nil defaults to true
	TopicFilter       string      `yaml:"topic_filter"`
	DeviceKeySource   string      `yaml:"device_key_source"`
	DeviceKeyPath     string      `yaml:"device_key_path"`
	Conditions        []Condition `yaml:"conditions"`
	ConditionLogic    string      `yaml:"condition_logic"`
	TimeWindow        *TimeWindow `yaml:"time_window"`
	CooldownMinutes   int         `yaml:"cooldown_minutes"`
	CooldownOnStartup bool        `yaml:"cooldown_on_startup"`
	ChatIDs           []int64     `yaml:"chat_ids"`
	MessageTemplate   string      `yaml:"message_template"`

	// Compiled fields (populated during Load).
	compiledTopicFilter *regexp.Regexp
}

// CompiledTopicFilter returns the pre-compiled topic regex, or nil if none was set.
func (r *Rule) CompiledTopicFilter() *regexp.Regexp {
	return r.compiledTopicFilter
}

// IsEnabled returns true if the rule is enabled. Defaults to true when not explicitly set.
func (r *Rule) IsEnabled() bool {
	if r.Enabled == nil {
		return true // Default: enabled.
	}
	return *r.Enabled
}

// Condition is a single comparison: extract a field from the JSON payload
// and compare it against a threshold value.
type Condition struct {
	FieldPath string `yaml:"field_path"`
	Operator  string `yaml:"operator"`
	Value     any    `yaml:"value"`
	ValueType string `yaml:"value_type"`
}

// TimeWindow restricts rule evaluation to a daily time range.
type TimeWindow struct {
	Start    string `yaml:"start"`
	End      string `yaml:"end"`
	Timezone string `yaml:"timezone"`

	// Compiled fields.
	startMinutes int
	endMinutes   int
	location     *time.Location
}

// StartMinutes returns the window start in minutes since midnight (local timezone).
func (tw *TimeWindow) StartMinutes() int { return tw.startMinutes }

// EndMinutes returns the window end in minutes since midnight (local timezone).
func (tw *TimeWindow) EndMinutes() int { return tw.endMinutes }

// Location returns the resolved time.Location for this window.
func (tw *TimeWindow) Location() *time.Location { return tw.location }

// InWindow returns true if the given time (in the window's location) falls within the window.
func (tw *TimeWindow) InWindow(now time.Time) bool {
	t := now.In(tw.location)
	current := t.Hour()*60 + t.Minute()

	if tw.startMinutes <= tw.endMinutes {
		// Normal window: e.g. 07:00–23:00.
		return current >= tw.startMinutes && current < tw.endMinutes
	}
	// Overnight window: e.g. 22:00–06:00.
	return current >= tw.startMinutes || current < tw.endMinutes
}

// validOperators is the set of allowed comparison operators.
var validOperators = map[string]bool{
	"eq": true, "ne": true,
	"gt": true, "gte": true,
	"lt": true, "lte": true,
}

// validValueTypes is the set of allowed value types for conditions.
var validValueTypes = map[string]bool{
	"number": true, "string": true, "boolean": true,
}

// Load reads, parses, validates and compiles a YAML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	if err := cfg.applyDefaults(); err != nil {
		return nil, err
	}

	cfg.overrideFromEnv()

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if err := cfg.compile(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// applyDefaults fills in sensible zero-value defaults.
func (c *Config) applyDefaults() error {
	if c.MQTT.ClientID == "" {
		hostname, _ := os.Hostname()
		c.MQTT.ClientID = fmt.Sprintf("smarthome-bridge-%s-%d", hostname, os.Getpid())
	}
	if c.MQTT.KeepAliveSec <= 0 {
		c.MQTT.KeepAliveSec = 60
	}
	if c.MQTT.ConnectTimeoutSec <= 0 {
		c.MQTT.ConnectTimeoutSec = 30
	}
	if c.MQTT.PingTimeoutSec <= 0 {
		c.MQTT.PingTimeoutSec = 10
	}

	if c.Telegram.APIBaseURL == "" {
		c.Telegram.APIBaseURL = "https://api.telegram.org"
	}
	if c.Telegram.RetryMax <= 0 {
		c.Telegram.RetryMax = 3
	}
	if len(c.Telegram.RetryBackoffMs) == 0 {
		c.Telegram.RetryBackoffMs = []int{200, 600, 1800}
	}

	if c.Engine.WorkerCount <= 0 {
		c.Engine.WorkerCount = 4
	}
	if c.Engine.InboundQueueSize <= 0 {
		c.Engine.InboundQueueSize = 256
	}
	if c.Engine.NotifyQueueSize <= 0 {
		c.Engine.NotifyQueueSize = 64
	}

	for i := range c.Rules {
		r := &c.Rules[i]
		if r.DeviceKeySource == "" {
			r.DeviceKeySource = "topic"
		}
		if r.ConditionLogic == "" {
			r.ConditionLogic = "and"
		}
	}
	return nil
}

// overrideFromEnv overrides config values from environment variables.
// *_FILE variables point to Docker secrets / mounted files.
func (c *Config) overrideFromEnv() {
	if v := envOrFile("MQTT_BROKER", "MQTT_BROKER_FILE", ""); v != "" {
		c.MQTT.Broker = v
	}
	if v := envOrFile("MQTT_USERNAME", "MQTT_USERNAME_FILE", ""); v != "" {
		c.MQTT.Username = v
	}
	if v := envOrFile("MQTT_PASSWORD", "MQTT_PASSWORD_FILE", ""); v != "" {
		c.MQTT.Password = v
	}
	if v := envOrFile("TELEGRAM_BOT_TOKEN", "TELEGRAM_BOT_TOKEN_FILE", ""); v != "" {
		c.Telegram.BotToken = v
	}
}

// validate checks all configuration constraints.
func (c *Config) validate() error {
	// MQTT validation.
	if c.MQTT.Broker == "" {
		return fmt.Errorf("mqtt.broker is required")
	}
	if !strings.HasPrefix(c.MQTT.Broker, "tcp://") && !strings.HasPrefix(c.MQTT.Broker, "ssl://") && !strings.HasPrefix(c.MQTT.Broker, "ws://") && !strings.HasPrefix(c.MQTT.Broker, "wss://") {
		return fmt.Errorf("mqtt.broker must start with tcp://, ssl://, ws://, or wss://")
	}
	if len(c.MQTT.Subscriptions) == 0 {
		return fmt.Errorf("mqtt.subscriptions must have at least one entry")
	}
	for i, sub := range c.MQTT.Subscriptions {
		if sub.Topic == "" {
			return fmt.Errorf("mqtt.subscriptions[%d].topic is required", i)
		}
		if sub.QoS > 2 {
			return fmt.Errorf("mqtt.subscriptions[%d].qos must be 0, 1, or 2", i)
		}
	}

	// Telegram validation.
	if c.Telegram.BotToken == "" {
		return fmt.Errorf("telegram.bot_token is required")
	}

	// Rules validation.
	ruleNames := make(map[string]bool)
	for i := range c.Rules {
		r := &c.Rules[i]
		if r.Name == "" {
			return fmt.Errorf("rules[%d].name is required", i)
		}
		if ruleNames[r.Name] {
			return fmt.Errorf("rules[%d].name %q is not unique", i, r.Name)
		}
		ruleNames[r.Name] = true

		if len(r.Conditions) == 0 {
			return fmt.Errorf("rule %q: conditions must have at least one entry", r.Name)
		}

		logic := strings.ToLower(r.ConditionLogic)
		if logic != "and" && logic != "or" {
			return fmt.Errorf("rule %q: condition_logic must be 'and' or 'or', got %q", r.Name, r.ConditionLogic)
		}
		r.ConditionLogic = logic // Normalize.

		src := strings.ToLower(r.DeviceKeySource)
		if src != "topic" && src != "field" && src != "rule" {
			return fmt.Errorf("rule %q: device_key_source must be 'topic', 'field', or 'rule', got %q", r.Name, r.DeviceKeySource)
		}
		r.DeviceKeySource = src

		if r.DeviceKeySource == "field" && r.DeviceKeyPath == "" {
			return fmt.Errorf("rule %q: device_key_path is required when device_key_source is 'field'", r.Name)
		}

		for j, cond := range r.Conditions {
			if cond.FieldPath == "" {
				return fmt.Errorf("rule %q: conditions[%d].field_path is required", r.Name, j)
			}
			if !validOperators[cond.Operator] {
				return fmt.Errorf("rule %q: conditions[%d].operator %q is invalid; must be one of: eq, ne, gt, gte, lt, lte", r.Name, j, cond.Operator)
			}
			if !validValueTypes[cond.ValueType] {
				return fmt.Errorf("rule %q: conditions[%d].value_type %q is invalid; must be one of: number, string, boolean", r.Name, j, cond.ValueType)
			}
		}

		if r.TimeWindow != nil {
			if err := validateTimeWindow(r.TimeWindow, r.Name); err != nil {
				return err
			}
		}

		if r.CooldownMinutes < 0 {
			return fmt.Errorf("rule %q: cooldown_minutes must be >= 0", r.Name)
		}

		if len(r.ChatIDs) == 0 {
			return fmt.Errorf("rule %q: chat_ids must have at least one entry", r.Name)
		}

		if r.MessageTemplate == "" {
			return fmt.Errorf("rule %q: message_template is required", r.Name)
		}
	}

	return nil
}

// validateTimeWindow checks a time window's fields and populates compiled values.
func validateTimeWindow(tw *TimeWindow, ruleName string) error {
	start, err := parseTimeOfDay(tw.Start)
	if err != nil {
		return fmt.Errorf("rule %q: time_window.start: %w", ruleName, err)
	}
	end, err := parseTimeOfDay(tw.End)
	if err != nil {
		return fmt.Errorf("rule %q: time_window.end: %w", ruleName, err)
	}
	tw.startMinutes = start
	tw.endMinutes = end

	if tw.Timezone == "" {
		tw.Timezone = "Local"
	}

	loc, err := time.LoadLocation(tw.Timezone)
	if err != nil {
		return fmt.Errorf("rule %q: time_window.timezone %q is invalid: %w", ruleName, tw.Timezone, err)
	}
	tw.location = loc

	return nil
}

// parseTimeOfDay parses "HH:MM" into minutes since midnight.
func parseTimeOfDay(s string) (int, error) {
	var h, m int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m)
	if err != nil {
		return 0, fmt.Errorf("invalid time format %q: must be HH:MM", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid time %q: hours 0-23, minutes 0-59", s)
	}
	return h*60 + m, nil
}

// envOrFile returns the value of envVar if set.
// Falls back to reading the file pointed to by fileEnvVar (Docker secrets pattern).
// Returns defaultValue if neither is available.
func envOrFile(envVar, fileEnvVar, defaultValue string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}
	if path := os.Getenv(fileEnvVar); path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return defaultValue
}

// compile pre-compiles regex patterns and performs final setup.
func (c *Config) compile() error {
	for i := range c.Rules {
		r := &c.Rules[i]
		if r.TopicFilter != "" {
			re, err := regexp.Compile(r.TopicFilter)
			if err != nil {
				return fmt.Errorf("rule %q: invalid topic_filter regex: %w", r.Name, err)
			}
			r.compiledTopicFilter = re
		}
	}
	return nil
}
