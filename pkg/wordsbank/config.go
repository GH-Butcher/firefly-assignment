package wordsbank

import (
	"log/slog"
	"net/http"
	"os"
)

type Option func(*Config)

type Config struct {
	Url           string
	HttpClient    *http.Client
	Log           *slog.Logger
	MinWordLength int
}

func NewConfig(opts ...Option) *Config {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := &Config{
		Url:           "http://localhost:8080",
		HttpClient:    &http.Client{},
		Log:           log,
		MinWordLength: 3,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

func WithUrl(url string) Option {
	return func(cfg *Config) {
		cfg.Url = url
	}
}

func WithHttpClient(client *http.Client) Option {
	return func(cfg *Config) {
		cfg.HttpClient = client
	}
}

func WithLog(log *slog.Logger) Option {
	return func(cfg *Config) {
		cfg.Log = log
	}
}

func WithMinWordLength(minWordLength int) Option {
	return func(cfg *Config) {
		cfg.MinWordLength = minWordLength
	}
}
