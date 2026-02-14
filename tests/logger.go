package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/LooneY2K/common-pkg-svc/log/custom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Bytes()
}

func (s *safeBuffer) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Len()
}

func parseFirstJSONLine(t *testing.T, b []byte) map[string]interface{} {
	t.Helper()

	lines := bytes.Split(b, []byte("\n"))
	require.NotEmpty(t, lines)
	require.NotZero(t, len(lines[0]))

	var m map[string]interface{}
	err := json.Unmarshal(lines[0], &m)
	require.NoError(t, err)

	return m
}

func TestLogger_Info_Output(t *testing.T) {
	buf := &safeBuffer{}

	fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	logger := log.New(
		log.WithOutput(buf),
		log.WithLevel(log.Info),
		log.WithMode(log.JSON),
		log.WithTimeFunc(func() time.Time { return fixedTime }),
	)

	logger.Info("hello world")

	require.NotZero(t, buf.Len(), "log output should not be empty")

	result := parseFirstJSONLine(t, buf.Bytes())

	assert.Equal(t, "hello world", result["msg"])
	assert.Equal(t, "INFO", strings.TrimSpace(result["level"].(string)))
	assert.Equal(t, fixedTime.Format(time.RFC3339), result["time"])
}

func TestLogger_LevelFiltering(t *testing.T) {
	buf := &safeBuffer{}

	logger := log.New(
		log.WithOutput(buf),
		log.WithLevel(log.Error), // INFO should be filtered out
	)

	logger.Info("should not log")

	assert.Zero(t, buf.Len(), "info logs must not be written when level=ERROR")
}

func TestLogger_WithFields(t *testing.T) {
	buf := &safeBuffer{}

	logger := log.New(
		log.WithOutput(buf),
		log.WithLevel(log.Info),
		log.WithMode(log.JSON),
	)
	logger.Info("processing", log.String("service", "payment"))

	result := parseFirstJSONLine(t, buf.Bytes())

	assert.Equal(t, "processing", result["msg"])
	assert.Equal(t, "payment", result["service"])
}

func TestLogger_Concurrency(t *testing.T) {
	buf := &safeBuffer{}

	logger := log.New(
		log.WithOutput(buf),
		log.WithLevel(log.Info),
	)

	var wg sync.WaitGroup
	count := 100

	wg.Add(count)

	for i := 0; i < count; i++ {
		go func() {
			defer wg.Done()
			logger.Info("concurrent log")
		}()
	}

	wg.Wait()

	assert.NotZero(t, buf.Len(), "logs should be written under concurrency")
}

func BenchmarkLogger_Info(b *testing.B) {
	logger := log.New(
		log.WithOutput(io.Discard),
		log.WithLevel(log.Info),
		log.WithMode(log.JSON),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
}
