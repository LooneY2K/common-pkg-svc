package log

import (
	"io"
	"time"
)

type Logger struct {
	out       io.Writer
	level     Level
	component string
	mode      Mode
	timeFn    func() time.Time
}

func New(opts ...Option) *Logger {
	l := defaultOptions()
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *Logger) Debug(msg string, fields ...Field) {
	l.log(Debug, msg, fields...)
}

func (l *Logger) Info(msg string, fields ...Field) {
	l.log(Info, msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...Field) {
	l.log(Warn, msg, fields...)
}

func (l *Logger) Error(msg string, fields ...Field) {
	l.log(Error, msg, fields...)
}

func (l *Logger) log(level Level, msg string, fields ...Field) {
	if level < l.level {
		return
	}

	switch l.mode {
	case JSON:
		l.logJSON(level, msg, fields...)
	default:
		l.logPretty(level, msg, fields...)
	}
}
