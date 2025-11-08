package logger

import (
	"log/slog"
	"os"
)

type Option func(*Config)

type Config struct {
	Log *slog.Logger
}

func Init(opts ...Option) *Config {
	cfg := &Config{
		slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func WithLogLevel(level slog.Level) Option {
	return func(cfg *Config) {
		cfg.Log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	}
}
