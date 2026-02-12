package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LooneY2K/common-pkg-svc/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create temp config file
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	err := os.WriteFile(path, []byte(`{"log":{"level":"info"},"port":8080}`), 0644)
	require.NoError(t, err)

	m, err := json.Load(path)
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.Equal(t, float64(8080), m["port"])
	log, ok := m["log"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "info", log["level"])
}

func TestLoad_NotFound(t *testing.T) {
	_, err := json.Load("/nonexistent/config.json")
	assert.Error(t, err)
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	err := os.WriteFile(path, []byte(`{invalid}`), 0644)
	require.NoError(t, err)

	_, err = json.Load(path)
	assert.Error(t, err)
}

func TestUnmarshal(t *testing.T) {
	data := []byte(`{"a":1,"b":"two"}`)
	m, err := json.Unmarshal(data)
	require.NoError(t, err)
	assert.Equal(t, float64(1), m["a"])
	assert.Equal(t, "two", m["b"])
}

func TestUnmarshalInto(t *testing.T) {
	m := map[string]any{
		"database": map[string]any{
			"host": "localhost",
			"port": float64(5432),
		},
	}
	var db struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	}
	err := json.UnmarshalInto(m, "database", &db)
	require.NoError(t, err)
	assert.Equal(t, "localhost", db.Host)
	assert.Equal(t, 5432, db.Port)
}
