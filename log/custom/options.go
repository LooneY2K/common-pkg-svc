package elog

import (
	"io"
	"os"
	"time"
)

type Option func(*Logger)

func WithOutput(w io.Writer) Option {
	return func(l *Logger) {
		l.out = w
	}
}

func WithLevel(level Level) Option {
	return func(l *Logger) {
		l.level = level
	}
}

func WithComponent(name string) Option {
	return func(l *Logger) {
		l.component = name
	}
}

func WithMode(mode Mode) Option {
	return func(l *Logger) {
		l.mode = mode
	}
}

func WithTimeFunc(fn func() time.Time) Option {
	return func(l *Logger) {
		l.timeFn = fn
	}
}

func defaultOptions() *Logger {
	return &Logger{
		out:    os.Stdout,
		level:  Info,
		mode:   Pretty,
		timeFn: time.Now,
	}
}
