package engine

import (
	"math"
	"strings"

	"github.com/smarthome-bridge/internal/config"
	"github.com/tidwall/gjson"
)

// evaluateCondition checks a single condition against a JSON payload.
// Returns true if the condition is satisfied, false otherwise.
func evaluateCondition(payload []byte, cond config.Condition) bool {
	result := gjson.GetBytes(payload, cond.FieldPath)
	if !result.Exists() {
		return false
	}

	switch strings.ToLower(cond.ValueType) {
	case "number":
		return compareNumbers(result, cond)
	case "string":
		return compareStrings(result, cond)
	case "boolean":
		return compareBools(result, cond)
	default:
		return false
	}
}

// compareNumbers compares numeric field values.
func compareNumbers(field gjson.Result, cond config.Condition) bool {
	fieldVal := field.Float()
	condVal, ok := toFloat64(cond.Value)
	if !ok {
		return false
	}

	switch cond.Operator {
	case "eq":
		return fieldVal == condVal
	case "ne":
		return fieldVal != condVal
	case "gt":
		return fieldVal > condVal
	case "gte":
		return fieldVal >= condVal
	case "lt":
		return fieldVal < condVal
	case "lte":
		return fieldVal <= condVal
	default:
		return false
	}
}

// compareStrings compares string field values.
func compareStrings(field gjson.Result, cond config.Condition) bool {
	fieldVal := field.String()
	condVal, ok := cond.Value.(string)
	if !ok {
		return false
	}

	switch cond.Operator {
	case "eq":
		return fieldVal == condVal
	case "ne":
		return fieldVal != condVal
	case "gt":
		return fieldVal > condVal
	case "gte":
		return fieldVal >= condVal
	case "lt":
		return fieldVal < condVal
	case "lte":
		return fieldVal <= condVal
	default:
		return false
	}
}

// compareBools compares boolean field values.
func compareBools(field gjson.Result, cond config.Condition) bool {
	fieldVal := field.Bool()
	condVal, ok := cond.Value.(bool)
	if !ok {
		return false
	}

	switch cond.Operator {
	case "eq":
		return fieldVal == condVal
	case "ne":
		return fieldVal != condVal
	default:
		fv, cv := boolToInt(fieldVal), boolToInt(condVal)
		switch cond.Operator {
		case "gt":
			return fv > cv
		case "gte":
			return fv >= cv
		case "lt":
			return fv < cv
		case "lte":
			return fv <= cv
		}
		return false
	}
}

// toFloat64 converts an interface{} to float64.
func toFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case int16:
		return float64(val), true
	case int8:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint64:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint8:
		return float64(val), true
	default:
		return math.NaN(), false
	}
}

// boolToInt converts bool to int for comparison (true=1, false=0).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// extractFields extracts all condition field values into a map for template rendering.
func extractFields(payload []byte, conditions []config.Condition) map[string]any {
	fields := make(map[string]any, len(conditions))
	for _, cond := range conditions {
		result := gjson.GetBytes(payload, cond.FieldPath)
		if result.Exists() {
			switch strings.ToLower(cond.ValueType) {
			case "number":
				fields[cond.FieldPath] = result.Float()
			case "string":
				fields[cond.FieldPath] = result.String()
			case "boolean":
				fields[cond.FieldPath] = result.Bool()
			default:
				fields[cond.FieldPath] = result.Value()
			}
		}
	}
	return fields
}

// extractDeviceKeyFromTopic extracts a device identifier from an MQTT topic.
// Uses the last segment of the topic (after the final '/').
func extractDeviceKeyFromTopic(topic string) string {
	idx := strings.LastIndexByte(topic, '/')
	if idx >= 0 && idx < len(topic)-1 {
		return topic[idx+1:]
	}
	return topic
}

// extractDeviceKeyFromPayload extracts a device identifier from the JSON payload using gjson path.
func extractDeviceKeyFromPayload(payload []byte, path string) string {
	result := gjson.GetBytes(payload, path)
	if !result.Exists() {
		return ""
	}
	return result.String()
}
