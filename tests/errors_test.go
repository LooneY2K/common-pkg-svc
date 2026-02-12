package main

import (
	"encoding/json"
	stderrors "errors"
	"testing"
	"time"

	"github.com/LooneY2K/common-pkg-svc/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrors_New(t *testing.T) {
	e := errors.New(errors.CodeInvalidInput, "internal debug", "user-facing message")
	require.NotNil(t, e)
	assert.Equal(t, "internal debug", e.Error())
	assert.Equal(t, errors.CodeInvalidInput, e.Code)
	assert.Equal(t, errors.KindValidation, e.Kind)
	assert.Equal(t, "user-facing message", e.UserMsg)
	assert.NotNil(t, e.StackTrace())
}

func TestErrors_New_DefaultUserMsg(t *testing.T) {
	e := errors.New(errors.CodeNotFound, "not found internally", "")
	assert.Equal(t, "not found internally", e.UserMsg)
}

func TestErrors_SentinelErrors(t *testing.T) {
	sentinels := map[string]*errors.AppError{
		"ErrInvalidInput": errors.ErrInvalidInput,
		"ErrNotFound":     errors.ErrNotFound,
		"ErrConflict":     errors.ErrConflict,
		"ErrTimeout":      errors.ErrTimeout,
		"ErrRateLimited":  errors.ErrRateLimited,
		"ErrForbidden":    errors.ErrForbidden,
		"ErrUnauthorized": errors.ErrUnauthorized,
		"ErrInternal":     errors.ErrInternal,
		"ErrNetwork":      errors.ErrNetwork,
	}
	for name, err := range sentinels {
		require.NotNil(t, err, "%s should not be nil", name)
		assert.NotEmpty(t, err.Error(), "%s should have message", name)
		assert.NotEmpty(t, string(err.Code), "%s should have code", name)
	}
}

func TestErrors_Wrap(t *testing.T) {
	base := errors.New(errors.CodeInternal, "base", "base")
	wrapped := errors.Wrap(base, "wrapped context")
	require.NotNil(t, wrapped)
	assert.Contains(t, wrapped.Error(), "wrapped context")
	assert.True(t, errors.Is(wrapped, base))
	assert.Equal(t, base, errors.Unwrap(wrapped))
}

func TestErrors_Wrap_Nil(t *testing.T) {
	assert.Nil(t, errors.Wrap(nil, "msg"))
}

func TestErrors_Wrapf(t *testing.T) {
	base := errors.New(errors.CodeNotFound, "not found", "not found")
	wrapped := errors.Wrapf(base, "resource %s", "user-123")
	require.NotNil(t, wrapped)
	assert.Contains(t, wrapped.Error(), "resource user-123")
}

func TestErrors_RootCause(t *testing.T) {
	inner := errors.New(errors.CodeValidation, "innermost", "innermost")
	mid := errors.Wrap(inner, "mid")
	outer := errors.Wrap(mid, "outer")
	assert.Equal(t, inner, errors.RootCause(outer))
	assert.Equal(t, inner, errors.RootCause(inner))
}

func TestErrors_NewFromError(t *testing.T) {
	stdErr := stderrors.New("stdlib error")
	ae := errors.NewFromError(stdErr, errors.CodeInternal, "something went wrong")
	require.NotNil(t, ae)
	assert.Equal(t, errors.CodeInternal, ae.Code)
	assert.Equal(t, "something went wrong", ae.UserMsg)
	assert.Equal(t, stdErr, ae.Cause)
}

func TestErrors_NewFromError_Nil(t *testing.T) {
	assert.Nil(t, errors.NewFromError(nil, errors.CodeInternal, "msg"))
}

func TestErrors_NewFromError_AppErrorPassthrough(t *testing.T) {
	orig := errors.New(errors.CodeNotFound, "orig", "orig")
	ae := errors.NewFromError(orig, errors.CodeInternal, "other")
	assert.Equal(t, orig, ae)
}

func TestErrors_HTTPStatus(t *testing.T) {
	cases := []struct {
		kind     errors.Kind
		expected int
	}{
		{errors.KindValidation, 400},
		{errors.KindUnauthorized, 401},
		{errors.KindAuth, 403},
		{errors.KindForbidden, 403},
		{errors.KindNotFound, 404},
		{errors.KindConflict, 409},
		{errors.KindTimeout, 408},
		{errors.KindRateLimit, 429},
		{errors.KindInternal, 500},
		{errors.KindNetwork, 500},
	}
	for _, c := range cases {
		e := errors.New(errors.CodeInternal, "msg", "msg", errors.WithKind(c.kind))
		assert.Equal(t, c.expected, e.HTTPStatus(), "kind %s", c.kind)
	}
}

func TestErrors_Options(t *testing.T) {
	cause := stderrors.New("cause")
	e := errors.New(errors.CodeInternal, "internal", "user",
		errors.WithCause(cause),
		errors.WithKind(errors.KindNetwork),
		errors.WithTraceID("trace-1"),
		errors.WithRequestID("req-1"),
		errors.WithRetryable(true),
		errors.WithTimeout(true),
		errors.WithMetadata(map[string]any{"key": "val"}),
	)
	assert.Equal(t, cause, e.Cause)
	assert.Equal(t, errors.KindNetwork, e.Kind)
	assert.Equal(t, "trace-1", e.TraceID)
	assert.Equal(t, "req-1", e.RequestID)
	assert.True(t, e.IsRetryable())
	assert.True(t, e.IsTimeout())
	assert.Equal(t, "val", e.Metadata["key"])
}

func TestErrors_MarshalJSON(t *testing.T) {
	e := errors.New(errors.CodeValidation, "internal", "invalid input",
		errors.WithTraceID("tid"),
		errors.WithRequestID("rid"),
	)
	e.Timestamp = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	b, err := json.Marshal(e)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	assert.Equal(t, "VALIDATION", m["code"])
	assert.Equal(t, "validation", m["kind"])
	assert.Equal(t, "invalid input", m["message"])
	assert.Equal(t, "tid", m["trace_id"])
	assert.Equal(t, "rid", m["request_id"])
	assert.Equal(t, "2025-01-01T00:00:00Z", m["timestamp"])
}

func TestErrors_MarshalJSON_RedactsPII(t *testing.T) {
	e := errors.New(errors.CodeValidation, "internal", "user",
		errors.WithMetadata(map[string]any{
			"password": "secret123",
			"email":    "user@example.com",
			"user_id":  "u1",
		}),
	)
	b, err := json.Marshal(e)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	meta, ok := m["metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "[REDACTED]", meta["password"])
	assert.Equal(t, "[REDACTED]", meta["email"])
	assert.Equal(t, "u1", meta["user_id"])
}

func TestErrors_UnmarshalJSON(t *testing.T) {
	data := []byte(`{"code":"NOT_FOUND","kind":"not_found","message":"resource not found","trace_id":"t1"}`)
	var e errors.AppError
	require.NoError(t, json.Unmarshal(data, &e))
	assert.Equal(t, errors.CodeNotFound, e.Code)
	assert.Equal(t, errors.KindNotFound, e.Kind)
	assert.Equal(t, "resource not found", e.UserMsg)
	assert.Equal(t, "t1", e.TraceID)
}

func TestErrors_LimitError(t *testing.T) {
	reset := time.Now().Add(time.Minute)
	le := errors.NewLimitError(5, reset)
	require.NotNil(t, le)
	assert.Equal(t, 5, le.Remaining)
	assert.Equal(t, reset, le.Reset)
	assert.Equal(t, errors.CodeRateLimited, le.Code)
}

func TestErrors_RBACError(t *testing.T) {
	rbac := errors.NewRBACError("viewer", "delete", "documents")
	require.NotNil(t, rbac)
	assert.Equal(t, "viewer", rbac.Role)
	assert.Equal(t, "delete", rbac.Action)
	assert.Equal(t, "documents", rbac.Resource)
	assert.Equal(t, errors.KindForbidden, rbac.Kind)
}

func TestErrors_FieldErrorf(t *testing.T) {
	fe := errors.FieldErrorf("email", "must be valid format")
	require.NotNil(t, fe)
	assert.Equal(t, "email", fe.Field)
	assert.Equal(t, "must be valid format", fe.Reason)
	assert.Equal(t, errors.KindValidation, fe.Kind)
}

func TestErrors_TimeoutError(t *testing.T) {
	deadline := time.Now().Add(30 * time.Second)
	te := errors.NewTimeoutError(deadline)
	require.NotNil(t, te)
	assert.Equal(t, deadline, te.Deadline)
	assert.True(t, te.IsTimeout())
	assert.True(t, te.IsRetryable())
}

func TestErrors_MultiError(t *testing.T) {
	e1 := errors.ErrNotFound
	e2 := errors.ErrInvalidInput
	merged := errors.MultiError(e1, e2)
	require.NotNil(t, merged)
	assert.True(t, errors.Is(merged, e1))
	assert.True(t, errors.Is(merged, e2))
}

func TestErrors_MultiError_NilAndSingle(t *testing.T) {
	assert.Nil(t, errors.MultiError())
	assert.Nil(t, errors.MultiError(nil, nil))
	assert.Equal(t, errors.ErrNotFound, errors.MultiError(errors.ErrNotFound))
}

func TestErrors_PublicError(t *testing.T) {
	ae := errors.New(errors.CodeInternal, "internal secret", "user safe")
	assert.Equal(t, "user safe", errors.PublicError(ae))
}

func TestErrors_PublicError_GenericFallback(t *testing.T) {
	stdErr := stderrors.New("raw stdlib error")
	assert.Equal(t, "an error occurred", errors.PublicError(stdErr))
}

func TestErrors_PublicError_Nil(t *testing.T) {
	assert.Empty(t, errors.PublicError(nil))
}

func TestErrors_IsRetryable(t *testing.T) {
	e := errors.New(errors.CodeTimeout, "timeout", "timeout", errors.WithRetryable(true))
	assert.True(t, errors.IsRetryable(e))
	te := errors.NewTimeoutError(time.Now())
	assert.True(t, errors.IsRetryable(te))
	assert.False(t, errors.IsRetryable(errors.ErrNotFound))
}

func TestErrors_IsTimeoutErr(t *testing.T) {
	assert.True(t, errors.IsTimeoutErr(errors.ErrTimeout))
	te := errors.NewTimeoutError(time.Now())
	assert.True(t, errors.IsTimeoutErr(te))
	assert.False(t, errors.IsTimeoutErr(errors.ErrNotFound))
}

func TestErrors_LogLevel(t *testing.T) {
	assert.Equal(t, errors.LogWarn, errors.ErrInvalidInput.LogLevel())
	assert.Equal(t, errors.LogError, errors.ErrInternal.LogLevel())
}
