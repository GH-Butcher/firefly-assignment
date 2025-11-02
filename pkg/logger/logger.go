package logger

import (
	"log/slog"
	"os"
)

type Logger struct {
	Log *slog.Logger
}

type Option func(*Logger)

func Init(opts ...Option) *Logger {
	l := &Logger{
		Log: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

func WithLevel(level slog.Level) Option {
	return func(l *Logger) {
		l.Log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	}
}
