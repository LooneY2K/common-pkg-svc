// Package json provides JSON file loading and unmarshaling into maps.
// Use an import alias to avoid conflict with encoding/json, e.g.:
//
//	import cfgjson "github.com/LooneY2K/common-pkg-svc/json"
package json

import (
	stdjson "encoding/json"
	"fmt"
	"os"
)

// Load reads a JSON file from path and unmarshals it into map[string]any.
// Returns an error if the file cannot be read or the content is invalid JSON.
func Load(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var m map[string]any
	if err := stdjson.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse config JSON: %w", err)
	}

	if m == nil {
		return make(map[string]any), nil
	}
	return m, nil
}

// Unmarshal unmarshals raw JSON bytes into map[string]any.
func Unmarshal(data []byte) (map[string]any, error) {
	var m map[string]any
	if err := stdjson.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if m == nil {
		return make(map[string]any), nil
	}
	return m, nil
}

// UnmarshalInto unmarshals a subset of the map into v.
// key is a dot-separated path (e.g. "database.host").
// If key is empty, the entire map is unmarshaled into v.
// v must be a pointer to a struct or map.
func UnmarshalInto(m map[string]any, key string, v any) error {
	var target any = m
	if key != "" {
		var ok bool
		target, ok = getNested(m, key)
		if !ok {
			return fmt.Errorf("key not found: %s", key)
		}
	}

	data, err := stdjson.Marshal(target)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := stdjson.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal into %T: %w", v, err)
	}
	return nil
}

// getNested retrieves a value by dot-separated key path.
func getNested(m map[string]any, key string) (any, bool) {
	if m == nil || key == "" {
		return nil, false
	}
	parts := splitKey(key)
	current := any(m)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		val, ok := m[part]
		if !ok {
			return nil, false
		}
		current = val
	}
	return current, true
}

func splitKey(key string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(key); i++ {
		if i == len(key) || key[i] == '.' {
			if i > start {
				parts = append(parts, key[start:i])
			}
			start = i + 1
		}
	}
	return parts
}
