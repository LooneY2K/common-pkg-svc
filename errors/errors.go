package errors

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
	"time"
)

// Ensure we use the standard library's errors for Join, Is, As, Unwrap
var (
	_ interface{ Unwrap() error } = (*AppError)(nil)
	_ interface{ Unwrap() error } = (*wrapError)(nil)
)

// Kind represents error categories for classification and metrics.
type Kind string

const (
	KindValidation   Kind = "validation"
	KindNetwork      Kind = "network"
	KindTimeout      Kind = "timeout"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindForbidden    Kind = "forbidden"
	KindInternal     Kind = "internal"
	KindRateLimit    Kind = "rate_limit"
	KindAuth         Kind = "auth"
	KindUnauthorized Kind = "unauthorized"
	KindOther        Kind = "other"
)

// Code represents machine-readable error codes.
type Code string

const (
	CodeInvalidInput     Code = "INVALID_INPUT"
	CodeNotFound         Code = "NOT_FOUND"
	CodeConflict         Code = "CONFLICT"
	CodeTimeout          Code = "TIMEOUT"
	CodeRateLimited      Code = "RATE_LIMITED"
	CodePermissionDenied Code = "PERMISSION_DENIED"
	CodeValidation       Code = "VALIDATION"
	CodeVideoDecodeFail  Code = "VIDEO_DECODE_FAIL"
	CodeNetwork          Code = "NETWORK"
	CodeInternal         Code = "INTERNAL"
	CodeUnauthorized     Code = "UNAUTHORIZED"
)

// LogLevel for error logging.
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// Sentinel errors for common conditions.
var (
	ErrInvalidInput = NewSentinel(CodeInvalidInput, "invalid input", KindValidation)
	ErrNotFound     = NewSentinel(CodeNotFound, "resource not found", KindNotFound)
	ErrConflict     = NewSentinel(CodeConflict, "resource conflict", KindConflict)
	ErrTimeout      = NewSentinel(CodeTimeout, "operation timed out", KindTimeout)
	ErrRateLimited  = NewSentinel(CodeRateLimited, "rate limit exceeded", KindRateLimit)
	ErrForbidden    = NewSentinel(CodePermissionDenied, "permission denied", KindForbidden)
	ErrUnauthorized = NewSentinel(CodeUnauthorized, "unauthorized", KindUnauthorized)
	ErrInternal     = NewSentinel(CodeInternal, "internal error", KindInternal)
	ErrNetwork      = NewSentinel(CodeNetwork, "network error", KindNetwork)
)

// AppError is the main error type implementing all production features.
type AppError struct {
	// Core
	Code        Code
	Kind        Kind
	InternalMsg string // For logs; may contain sensitive data
	UserMsg     string // Safe for clients
	Cause       error
	stackTrace  []byte

	// Context
	TraceID   string
	RequestID string
	Timestamp time.Time

	// Behavior
	Retryable bool
	Timeout   bool

	// Dynamic
	Metadata map[string]any
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.InternalMsg != "" {
		return e.InternalMsg
	}
	if e.UserMsg != "" {
		return e.UserMsg
	}
	return string(e.Code)
}

// Unwrap returns the cause for Go 1.13+ error chains.
func (e *AppError) Unwrap() error { return e.Cause }

// StackTrace returns the captured stack trace.
func (e *AppError) StackTrace() []byte { return e.stackTrace }

// HTTPStatus returns the appropriate HTTP status code.
func (e *AppError) HTTPStatus() int {
	return kindToHTTPStatus(e.Kind)
}

// ErrorKind returns the kind string for Prometheus metrics.
func (e *AppError) ErrorKind() string {
	return string(e.Kind)
}

// IsRetryable indicates whether the operation can be retried.
func (e *AppError) IsRetryable() bool {
	return e.Retryable
}

// IsTimeout indicates if this is a timeout error.
func (e *AppError) IsTimeout() bool {
	return e.Timeout || e.Kind == KindTimeout
}

// LogLevel returns the suggested log level.
func (e *AppError) LogLevel() LogLevel {
	switch e.Kind {
	case KindValidation, KindNotFound:
		return LogWarn
	case KindForbidden, KindAuth, KindRateLimit:
		return LogWarn
	case KindTimeout, KindNetwork:
		return LogWarn
	case KindInternal, KindConflict, KindOther:
		return LogError
	default:
		return LogError
	}
}

// StackFrames returns frames suitable for Sentry and similar SDKs.
func (e *AppError) StackFrames() []Frame {
	return parseStackFrames(e.stackTrace)
}

// MarshalJSON implements json.Marshaler with safe field exposure.
func (e *AppError) MarshalJSON() ([]byte, error) {
	m := map[string]any{
		"code":    string(e.Code),
		"kind":    string(e.Kind),
		"message": e.UserMsg,
	}
	if e.TraceID != "" {
		m["trace_id"] = e.TraceID
	}
	if e.RequestID != "" {
		m["request_id"] = e.RequestID
	}
	if !e.Timestamp.IsZero() {
		m["timestamp"] = e.Timestamp.Format(time.RFC3339)
	}
	if len(e.Metadata) > 0 {
		m["metadata"] = redactMetadata(e.Metadata)
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements json.Unmarshaler (partial; for client responses).
func (e *AppError) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if c, ok := m["code"].(string); ok {
		e.Code = Code(c)
	}
	if k, ok := m["kind"].(string); ok {
		e.Kind = Kind(k)
	}
	if msg, ok := m["message"].(string); ok {
		e.UserMsg = msg
		e.InternalMsg = msg
	}
	if tid, ok := m["trace_id"].(string); ok {
		e.TraceID = tid
	}
	if rid, ok := m["request_id"].(string); ok {
		e.RequestID = rid
	}
	if ts, ok := m["timestamp"].(string); ok {
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
	}
	if meta, ok := m["metadata"].(map[string]any); ok {
		e.Metadata = meta
	}
	return nil
}

// wrapError wraps an error with a message, preserving the chain.
type wrapError struct {
	msg   string
	cause error
}

func (e *wrapError) Error() string {
	if e.cause != nil {
		return e.msg + ": " + e.cause.Error()
	}
	return e.msg
}

func (e *wrapError) Unwrap() error { return e.cause }

func kindToHTTPStatus(k Kind) int {
	switch k {
	case KindValidation:
		return 400
	case KindAuth, KindForbidden:
		return 403
	case KindUnauthorized:
		return 401
	case KindNotFound:
		return 404
	case KindConflict:
		return 409
	case KindTimeout:
		return 408
	case KindRateLimit:
		return 429
	case KindNetwork, KindInternal, KindOther:
		return 500
	default:
		return 500
	}
}

// Frame represents a stack frame for Sentry etc.
type Frame struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Func string `json:"function"`
}

func parseStackFrames(b []byte) []Frame {
	if len(b) == 0 {
		return nil
	}
	// Parse debug.Stack() format: "goroutine N [state]:\nfunc(...)\n\t/path/file.go:line +0x..."
	lines := strings.Split(string(b), "\n")
	var frames []Frame
	for i := 1; i < len(lines); i += 2 {
		if i+1 >= len(lines) {
			break
		}
		funcLine := strings.TrimSpace(lines[i])
		fileLine := strings.TrimSpace(lines[i+1])
		f := Frame{Func: funcLine}
		if idx := strings.LastIndex(fileLine, ":"); idx >= 0 {
			f.File = strings.TrimPrefix(fileLine[:idx], "\t")
			fmt.Sscanf(fileLine[idx+1:], "%d", &f.Line)
		}
		frames = append(frames, f)
	}
	if len(frames) == 0 {
		return []Frame{{File: "unknown", Line: 0, Func: "unknown"}}
	}
	return frames
}

func redactMetadata(m map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range m {
		if isPIIField(k) {
			out[k] = "[REDACTED]"
		} else {
			out[k] = v
		}
	}
	return out
}

var pIIFields = map[string]bool{
	"password": true, "token": true, "secret": true, "email": true,
	"ssn": true, "credit_card": true, "api_key": true,
}

func isPIIField(k string) bool {
	lower := strings.ToLower(k)
	return pIIFields[lower] || strings.Contains(lower, "password") ||
		strings.Contains(lower, "token") || strings.Contains(lower, "secret")
}

// New creates an AppError with optional cause and metadata.
func New(code Code, internalMsg, userMsg string, opts ...Option) *AppError {
	e := &AppError{
		Code:        code,
		Kind:        codeToKind(code),
		InternalMsg: internalMsg,
		UserMsg:     userMsg,
		Timestamp:   time.Now(),
		Metadata:    make(map[string]any),
	}
	if userMsg == "" {
		e.UserMsg = internalMsg
	}
	e.stackTrace = debug.Stack()
	for _, o := range opts {
		o(e)
	}
	return e
}

// Option configures an AppError.
type Option func(*AppError)

func WithCause(cause error) Option {
	return func(e *AppError) { e.Cause = cause }
}

func WithKind(k Kind) Option {
	return func(e *AppError) { e.Kind = k }
}

func WithTraceID(id string) Option {
	return func(e *AppError) { e.TraceID = id }
}

func WithRequestID(id string) Option {
	return func(e *AppError) { e.RequestID = id }
}

func WithRetryable(retryable bool) Option {
	return func(e *AppError) { e.Retryable = retryable }
}

func WithTimeout(isTimeout bool) Option {
	return func(e *AppError) { e.Timeout = isTimeout }
}

func WithMetadata(m map[string]any) Option {
	return func(e *AppError) {
		if e.Metadata == nil {
			e.Metadata = make(map[string]any)
		}
		for k, v := range m {
			e.Metadata[k] = v
		}
	}
}

func codeToKind(c Code) Kind {
	switch c {
	case CodeInvalidInput, CodeValidation:
		return KindValidation
	case CodeNotFound:
		return KindNotFound
	case CodeConflict:
		return KindConflict
	case CodeTimeout:
		return KindTimeout
	case CodeRateLimited:
		return KindRateLimit
	case CodePermissionDenied, CodeUnauthorized:
		return KindAuth
	case CodeNetwork:
		return KindNetwork
	case CodeInternal, CodeVideoDecodeFail:
		return KindInternal
	default:
		return KindOther
	}
}

// NewSentinel creates a sentinel AppError (no stack, no timestamp).
func NewSentinel(code Code, msg string, kind Kind) *AppError {
	return &AppError{
		Code:        code,
		Kind:        kind,
		InternalMsg: msg,
		UserMsg:     msg,
	}
}

// NewFromError wraps an existing error as AppError.
func NewFromError(err error, code Code, userMsg string) *AppError {
	if err == nil {
		return nil
	}
	if ae, ok := err.(*AppError); ok {
		return ae
	}
	return New(code, err.Error(), userMsg, WithCause(err))
}

// Wrap wraps an error with a message, preserving the chain.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &wrapError{msg: msg, cause: err}
}

// Wrapf wraps with format.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return Wrap(err, fmt.Sprintf(format, args...))
}

// RootCause traverses the error chain and returns the deepest cause.
func RootCause(err error) error {
	if err == nil {
		return nil
	}
	for {
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		next := u.Unwrap()
		if next == nil {
			return err
		}
		err = next
	}
}

// Is reports whether any error in the chain matches target (Go 1.13+).
func Is(err, target error) bool {
	return errors.Is(err, target)
}

// As finds the first error that matches target (Go 1.13+).
func As(err error, target any) bool {
	return errors.As(err, target)
}

// Unwrap returns the result of calling Unwrap on err (Go 1.13+).
func Unwrap(err error) error {
	return errors.Unwrap(err)
}

// LimitError represents rate limit errors with retry info.
type LimitError struct {
	AppError
	Remaining int
	Reset     time.Time
}

// NewLimitError creates a rate limit error.
func NewLimitError(remaining int, reset time.Time, opts ...Option) *LimitError {
	e := New(CodeRateLimited, "rate limit exceeded", "rate limit exceeded", opts...)
	e.Kind = KindRateLimit
	return &LimitError{
		AppError:  *e,
		Remaining: remaining,
		Reset:     reset,
	}
}

// RBACError represents permission/authorization errors.
type RBACError struct {
	AppError
	Role     string
	Action   string
	Resource string
}

// NewRBACError creates a permission error.
func NewRBACError(role, action, resource string, opts ...Option) *RBACError {
	e := New(CodePermissionDenied,
		fmt.Sprintf("permission denied: %s cannot %s on %s", role, action, resource),
		"permission denied",
		opts...)
	e.Kind = KindForbidden
	return &RBACError{
		AppError: *e,
		Role:     role,
		Action:   action,
		Resource: resource,
	}
}

// FieldError represents a validation error for a specific field.
type FieldError struct {
	AppError
	Field  string
	Reason string
}

// FieldErrorf creates a field validation error.
func FieldErrorf(field, format string, args ...any) *FieldError {
	reason := fmt.Sprintf(format, args...)
	e := New(CodeValidation,
		fmt.Sprintf("field %s: %s", field, reason),
		fmt.Sprintf("invalid %s: %s", field, reason))
	e.Kind = KindValidation
	return &FieldError{
		AppError: *e,
		Field:    field,
		Reason:   reason,
	}
}

// TimeoutError wraps timeout with deadline.
type TimeoutError struct {
	AppError
	Deadline time.Time
}

// NewTimeoutError creates a timeout error.
func NewTimeoutError(deadline time.Time, opts ...Option) *TimeoutError {
	e := New(CodeTimeout, "operation timed out", "request timed out",
		append(opts, WithTimeout(true), WithRetryable(true))...)
	e.Kind = KindTimeout
	return &TimeoutError{
		AppError: *e,
		Deadline: deadline,
	}
}

// MultiError aggregates multiple errors (wraps std errors.Join).
func MultiError(errs ...error) error {
	var nonNil []error
	for _, e := range errs {
		if e != nil {
			nonNil = append(nonNil, e)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	if len(nonNil) == 1 {
		return nonNil[0]
	}
	return errors.Join(nonNil...)
}

// IsRetryable reports whether err (or any error in its chain) is retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.IsRetryable()
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		return te.IsRetryable()
	}
	return false
}

// IsTimeoutErr reports whether err (or any in chain) is a timeout error.
func IsTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var ae *AppError
	if errors.As(err, &ae) {
		return ae.IsTimeout()
	}
	var te *TimeoutError
	if errors.As(err, &te) {
		return te.IsTimeout()
	}
	// Check for context.DeadlineExceeded etc.
	return errors.Is(err, ErrTimeout)
}

// PublicError returns a client-safe string representation of the error.
func PublicError(err error) string {
	if err == nil {
		return ""
	}
	if ae, ok := err.(*AppError); ok && ae.UserMsg != "" {
		return ae.UserMsg
	}
	if fe, ok := err.(*FieldError); ok {
		return fe.UserMsg
	}
	if le, ok := err.(*LimitError); ok {
		return le.UserMsg
	}
	if te, ok := err.(*TimeoutError); ok {
		return te.UserMsg
	}
	if rbac, ok := err.(*RBACError); ok {
		return rbac.UserMsg
	}
	// Fallback: generic message to avoid leaking internals
	return "an error occurred"
}
