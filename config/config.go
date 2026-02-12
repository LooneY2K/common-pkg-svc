// Package config provides production-ready configuration loading with
// JSON file support, environment variable overrides, and type-safe access.
package config

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/LooneY2K/common-pkg-svc/converter"
	cfgjson "github.com/LooneY2K/common-pkg-svc/json"
)

// Config holds configuration values with support for nested keys and env overrides.
// Safe for concurrent read after Load.
type Config struct {
	mu        sync.RWMutex
	data      map[string]any
	envPrefix string
}

// Option configures Config behavior.
type Option func(*Config)

// WithEnvPrefix sets the prefix for environment variable overrides.
// Keys like "log.level" become "PREFIX_LOG_LEVEL" (uppercase, dots → underscores).
// Example: WithEnvPrefix("APP") makes APP_LOG_LEVEL override log.level.
func WithEnvPrefix(prefix string) Option {
	return func(c *Config) {
		c.envPrefix = strings.TrimSuffix(strings.ToUpper(prefix), "_")
	}
}

// New creates an empty Config with optional settings.
func New(opts ...Option) *Config {
	c := &Config{
		data: make(map[string]any),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Load reads configuration from a JSON file and returns a Config.
// Returns an error if the file cannot be read or parsed.
func Load(path string, opts ...Option) (*Config, error) {
	m, err := cfgjson.Load(path)
	if err != nil {
		return nil, fmt.Errorf("config load: %w", err)
	}
	return FromMap(m, opts...)
}

// FromMap creates a Config from an existing map.
func FromMap(m map[string]any, opts ...Option) (*Config, error) {
	if m == nil {
		m = make(map[string]any)
	}
	c := New(opts...)
	c.mu.Lock()
	c.data = deepCopy(m)
	c.mu.Unlock()
	return c, nil
}

// Get retrieves a value by dot-separated key (e.g. "log.level").
// Environment variable overrides are checked first when env prefix is set.
// Returns (nil, false) if the key is not found.
func (c *Config) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check env override first
	if c.envPrefix != "" {
		envKey := c.envKey(key)
		if v, ok := os.LookupEnv(envKey); ok {
			return v, true
		}
	}

	return getNested(c.data, key)
}

// GetString returns the string value for key.
// Returns empty string if not found or type conversion fails.
func (c *Config) GetString(key string) string {
	v, _ := c.Get(key)
	return converter.ToString(v)
}

// GetInt returns the int value for key.
// Returns 0 if not found or conversion fails.
func (c *Config) GetInt(key string) int {
	v, _ := c.Get(key)
	return converter.ToInt(v)
}

// GetInt64 returns the int64 value for key.
func (c *Config) GetInt64(key string) int64 {
	v, _ := c.Get(key)
	return converter.ToInt64(v)
}

// GetBool returns the bool value for key.
// Returns false if not found or conversion fails.
func (c *Config) GetBool(key string) bool {
	v, _ := c.Get(key)
	return converter.ToBool(v)
}

// GetFloat64 returns the float64 value for key.
func (c *Config) GetFloat64(key string) float64 {
	v, _ := c.Get(key)
	return converter.ToFloat64(v)
}

// GetDuration returns the time.Duration value for key.
func (c *Config) GetDuration(key string) time.Duration {
	v, _ := c.Get(key)
	return converter.ToDuration(v)
}

// GetOrDefault returns the value for key, or defaultVal if not found.
func (c *Config) GetOrDefault(key string, defaultVal any) any {
	v, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	return v
}

// GetStringOrDefault returns the string value, or defaultVal if not found.
func (c *Config) GetStringOrDefault(key string, defaultVal string) string {
	v, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	return converter.ToString(v)
}

// GetIntOrDefault returns the int value, or defaultVal if not found.
func (c *Config) GetIntOrDefault(key string, defaultVal int) int {
	v, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	return converter.ToInt(v)
}

// GetBoolOrDefault returns the bool value, or defaultVal if not found.
func (c *Config) GetBoolOrDefault(key string, defaultVal bool) bool {
	v, ok := c.Get(key)
	if !ok {
		return defaultVal
	}
	return converter.ToBool(v)
}

// Has returns true if the key exists.
func (c *Config) Has(key string) bool {
	_, ok := c.Get(key)
	return ok
}

// UnmarshalKey unmarshals a subset of config into v.
// key is a dot-separated path (e.g. "database").
// v must be a pointer to a struct or map.
func (c *Config) UnmarshalKey(key string, v any) error {
	c.mu.RLock()
	data := deepCopy(c.data)
	c.mu.RUnlock()

	return cfgjson.UnmarshalInto(data, key, v)
}

// Set assigns a value for key (dot-separated).
// Useful for programmatic overrides or defaults.
// Not thread-safe for concurrent writes; call before sharing Config.
func (c *Config) Set(key string, val any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	setNested(c.data, key, val)
}

func (c *Config) envKey(key string) string {
	s := strings.ReplaceAll(key, ".", "_")
	s = strings.ToUpper(s)
	if c.envPrefix != "" {
		return c.envPrefix + "_" + s
	}
	return s
}

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

func setNested(m map[string]any, key string, val any) {
	parts := splitKey(key)
	if len(parts) == 0 {
		return
	}
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := m[part].(map[string]any)
		if !ok || next == nil {
			next = make(map[string]any)
			m[part] = next
		}
		m = next
	}
	m[parts[len(parts)-1]] = val
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

func deepCopy(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if vm, ok := v.(map[string]any); ok {
			out[k] = deepCopy(vm)
		} else {
			out[k] = v
		}
	}
	return out
}
