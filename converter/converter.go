package converter

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ToString converts any value to string.
// Handles nil, bool, int, int64, float64, string.
// Returns empty string for unsupported types or nil.
func ToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case bool:
		return strconv.FormatBool(val)
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	default:
		return fmt.Sprint(v)
	}
}

// ToInt converts any value to int.
// Handles int, int64, float64, and numeric strings.
// Returns 0 for nil or unparseable values.
func ToInt(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case int32:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(val))
		return n
	default:
		n, _ := strconv.Atoi(ToString(v))
		return n
	}
}

// ToInt64 converts any value to int64.
func ToInt64(v any) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case int32:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(ToString(v), 10, 64)
		return n
	}
}

// ToBool converts any value to bool.
// Handles bool, "true"/"false" strings (case-insensitive), "1"/"0".
// Returns false for nil or unparseable values.
func ToBool(v any) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		s := strings.ToLower(strings.TrimSpace(val))
		return s == "true" || s == "1" || s == "yes" || s == "on"
	case int, int64:
		return ToInt(v) != 0
	case float64:
		return val != 0
	default:
		return ToString(v) == "true"
	}
}

// ToFloat64 converts any value to float64.
func ToFloat64(v any) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(val), 64)
		return f
	default:
		f, _ := strconv.ParseFloat(ToString(v), 64)
		return f
	}
}

// ToDuration converts any value to time.Duration.
// Accepts duration strings (e.g. "5s", "1m30s") and int/int64 as nanoseconds.
func ToDuration(v any) time.Duration {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case time.Duration:
		return val
	case string:
		d, _ := time.ParseDuration(strings.TrimSpace(val))
		return d
	case int64:
		return time.Duration(val)
	case int:
		return time.Duration(val)
	default:
		d, _ := time.ParseDuration(ToString(v))
		return d
	}
}
