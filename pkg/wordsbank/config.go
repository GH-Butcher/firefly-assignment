package wordsbank

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

const (
	minWordLength  = 3
	url            = "http://localhost:8080"
	defaultTimeout = 10 * time.Second
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
		Url:           url,
		Log:           log,
		MinWordLength: minWordLength,
	}

	if cfg.HttpClient == nil {
		cfg.HttpClient = &http.Client{
			Timeout: defaultTimeout,
		}
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
