package engine

import (
	"log/slog"
	"time"

	"github.com/smarthome-bridge/internal/config"
	"github.com/tidwall/gjson"
)

// TriggerResult is produced when a rule's conditions are met.
type TriggerResult struct {
	RuleName    string
	Description string
	Device      string
	Topic       string
	Fields      map[string]any
	RawPayload  []byte
	ChatIDs     []int64
	Template    string
	Timestamp   time.Time
}

// Evaluator checks incoming messages against rules and returns TriggerResult when matched.
type Evaluator struct {
	cooldown *CooldownTracker
	tmpl     *TemplateCache
}

// NewEvaluator creates a new Evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{
		cooldown: NewCooldownTracker(),
		tmpl:     NewTemplateCache(),
	}
}

// Evaluate checks a message against a single rule.
// Returns a TriggerResult if all conditions are met and windows/cooldown allow, or nil otherwise.
func (e *Evaluator) Evaluate(topic string, payload []byte, rule *config.Rule) *TriggerResult {
	// Step 1: Topic regex filter (if configured).
	if re := rule.CompiledTopicFilter(); re != nil {
		if !re.MatchString(topic) {
			return nil
		}
	}

	// Step 2: Check that payload is valid JSON.
	if !gjson.ValidBytes(payload) {
		slog.Warn("invalid JSON payload, skipping rule",
			"topic", topic,
			"rule", rule.Name,
		)
		return nil
	}

	// Step 3: Evaluate each condition.
	allMatch, anyMatch := true, false
	for _, cond := range rule.Conditions {
		ok := evaluateCondition(payload, cond)
		if rule.ConditionLogic == "and" {
			if !ok {
				allMatch = false
				break
			}
		} else { // "or"
			if ok {
				anyMatch = true
				break
			}
		}
	}

	matched := false
	if rule.ConditionLogic == "and" {
		matched = allMatch
	} else {
		matched = anyMatch
	}
	if !matched {
		return nil
	}

	// Step 4: Time window check.
	if rule.TimeWindow != nil {
		now := time.Now()
		if !rule.TimeWindow.InWindow(now) {
			return nil
		}
	}

	// Step 5: Extract device key for cooldown.
	deviceKey := e.deviceKey(topic, payload, rule)

	// Step 6: Cooldown check.
	if rule.CooldownMinutes > 0 && e.cooldown.IsOnCooldown(deviceKey, rule.CooldownMinutes) {
		return nil
	}

	// Step 7: Extract field values for template.
	fields := extractFields(payload, rule.Conditions)

	// Step 8: Update cooldown.
	if rule.CooldownMinutes > 0 {
		e.cooldown.Set(deviceKey)
	}

	return &TriggerResult{
		RuleName:    rule.Name,
		Description: rule.Description,
		Device:      deviceKey,
		Topic:       topic,
		Fields:      fields,
		RawPayload:  payload,
		ChatIDs:     rule.ChatIDs,
		Template:    rule.MessageTemplate,
		Timestamp:   time.Now(),
	}
}

// deviceKey extracts the device identifier used for cooldown grouping.
func (e *Evaluator) deviceKey(topic string, payload []byte, rule *config.Rule) string {
	var key string
	switch rule.DeviceKeySource {
	case "field":
		key = extractDeviceKeyFromPayload(payload, rule.DeviceKeyPath)
	case "rule":
		key = rule.Name
	default: // "topic"
		key = extractDeviceKeyFromTopic(topic)
	}
	// Make the key unique per rule.
	return rule.Name + ":" + key
}

// Cooldown returns the underlying cooldown tracker (for diagnostics).
func (e *Evaluator) Cooldown() *CooldownTracker {
	return e.cooldown
}

// TemplateCache returns the underlying template cache (for diagnostics).
func (e *Evaluator) TemplateCache() *TemplateCache {
	return e.tmpl
}

