package zap

import (
	"context"

	"go.uber.org/zap"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func (l *Logger) FromContext(ctx context.Context) *zap.Logger {
	if reqID, ok := ctx.Value(requestIDKey).(string); ok {
		return l.With(zap.String("request_id", reqID))
	}
	return l.Logger
}
