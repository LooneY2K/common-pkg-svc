package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LooneY2K/common-pkg-svc/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Load(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(`{"log":{"level":"info"},"server":{"port":8080}}`), 0644)
	require.NoError(t, err)

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "info", cfg.GetString("log.level"))
	assert.Equal(t, 8080, cfg.GetInt("server.port"))
}

func TestConfig_FromMap(t *testing.T) {
	m := map[string]any{
		"log":  map[string]any{"level": "debug"},
		"port": float64(3000),
	}
	cfg, err := config.FromMap(m)
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.GetString("log.level"))
	assert.Equal(t, 3000, cfg.GetInt("port"))
}

func TestConfig_GetNested(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{
			"b": map[string]any{"c": "value"},
		},
	}
	cfg, err := config.FromMap(m)
	require.NoError(t, err)
	v, ok := cfg.Get("a.b.c")
	assert.True(t, ok)
	assert.Equal(t, "value", v)
}

func TestConfig_GetOrDefault(t *testing.T) {
	cfg, err := config.FromMap(map[string]any{"x": "foo"})
	require.NoError(t, err)
	assert.Equal(t, "foo", cfg.GetStringOrDefault("x", "default"))
	assert.Equal(t, "default", cfg.GetStringOrDefault("missing", "default"))
	assert.Equal(t, 42, cfg.GetIntOrDefault("missing", 42))
}

func TestConfig_Has(t *testing.T) {
	cfg, err := config.FromMap(map[string]any{"present": true})
	require.NoError(t, err)
	assert.True(t, cfg.Has("present"))
	assert.False(t, cfg.Has("absent"))
}

func TestConfig_Set(t *testing.T) {
	cfg, err := config.FromMap(map[string]any{})
	require.NoError(t, err)
	cfg.Set("new.key", "value")
	assert.Equal(t, "value", cfg.GetString("new.key"))
}

func TestConfig_EnvOverride(t *testing.T) {
	cfg, err := config.FromMap(
		map[string]any{"log": map[string]any{"level": "info"}},
		config.WithEnvPrefix("APP"),
	)
	require.NoError(t, err)
	os.Setenv("APP_LOG_LEVEL", "debug")
	defer os.Unsetenv("APP_LOG_LEVEL")

	assert.Equal(t, "debug", cfg.GetString("log.level"))
}

func TestConfig_UnmarshalKey(t *testing.T) {
	m := map[string]any{
		"database": map[string]any{
			"host": "db.example.com",
			"port": float64(5432),
		},
	}
	cfg, err := config.FromMap(m)
	require.NoError(t, err)
	var db struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	err = cfg.UnmarshalKey("database", &db)
	require.NoError(t, err)
	assert.Equal(t, "db.example.com", db.Host)
	assert.Equal(t, 5432, db.Port)
}

// Example functions for godoc (run with go test -run Example).
func ExampleLoad() {
	dir, _ := os.MkdirTemp("", "config-example")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte(`{"log":{"level":"info"},"server":{"port":8080}}`), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		fmt.Println("load error:", err)
		return
	}

	fmt.Println(cfg.GetString("log.level"))
	fmt.Println(cfg.GetInt("server.port"))
	// Output:
	// info
	// 8080
}

func ExampleFromMap() {
	m := map[string]any{
		"database": map[string]any{
			"host": "localhost",
			"port": float64(5432),
		},
	}
	cfg, _ := config.FromMap(m)

	var db struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	_ = cfg.UnmarshalKey("database", &db)
	fmt.Println(db.Host, db.Port)
	// Output:
	// localhost 5432
}
