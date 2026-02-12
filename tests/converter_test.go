package main

import (
	"testing"
	"time"

	"github.com/LooneY2K/common-pkg-svc/converter"
	"github.com/stretchr/testify/assert"
)

func TestToString(t *testing.T) {
	assert.Equal(t, "hello", converter.ToString("hello"))
	assert.Equal(t, "42", converter.ToString(42))
	assert.Equal(t, "true", converter.ToString(true))
	assert.Equal(t, "", converter.ToString(nil))
}

func TestToInt(t *testing.T) {
	assert.Equal(t, 42, converter.ToInt(42))
	assert.Equal(t, 42, converter.ToInt(int64(42)))
	assert.Equal(t, 42, converter.ToInt("42"))
	assert.Equal(t, 0, converter.ToInt(nil))
	assert.Equal(t, 0, converter.ToInt("invalid"))
}

func TestToBool(t *testing.T) {
	assert.True(t, converter.ToBool(true))
	assert.True(t, converter.ToBool("true"))
	assert.True(t, converter.ToBool("TRUE"))
	assert.True(t, converter.ToBool("1"))
	assert.False(t, converter.ToBool(false))
	assert.False(t, converter.ToBool("false"))
	assert.False(t, converter.ToBool(nil))
}

func TestToDuration(t *testing.T) {
	assert.Equal(t, 5*time.Second, converter.ToDuration("5s"))
	assert.Equal(t, time.Minute+30*time.Second, converter.ToDuration("1m30s"))
	assert.Equal(t, int64(1e9), int64(converter.ToDuration(int64(1e9))))
}
