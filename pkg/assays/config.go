package assays

import "runtime"

type Option func(*Config)

type Config struct {
	MinWordLength  int
	MaxUrlsToFetch int
	UrlsListPath   string
	Workers        int
	Buffer         int
	RatePerSecond  int
	TopWordsCount  int
}

func NewConfig(opts ...Option) *Config {
	cfg := &Config{
		MinWordLength:  3,
		MaxUrlsToFetch: 0,
		Workers:        runtime.NumCPU(),
		Buffer:         128,
		RatePerSecond:  30,
		TopWordsCount:  10,
	}

	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

func WithMinWordLength(minWordLength int) Option {
	return func(cfg *Config) {
		cfg.MinWordLength = minWordLength
	}
}

func WithMaxUrlsToFetch(maxUrlsToFetch int) Option {
	return func(cfg *Config) {
		cfg.MaxUrlsToFetch = maxUrlsToFetch
	}
}

func WithUrlsListPath(urlsListPath string) Option {
	return func(cfg *Config) {
		cfg.UrlsListPath = urlsListPath
	}
}

func WithWorkers(workers int) Option {
	return func(cfg *Config) {
		cfg.Workers = workers
	}
}

func WithBuffer(buffer int) Option {
	return func(cfg *Config) {
		cfg.Buffer = buffer
	}
}

func WithRatePerSecond(ratePerSecond int) Option {
	return func(cfg *Config) {
		cfg.RatePerSecond = ratePerSecond
	}
}

func WithTopWordsCount(topWordsCount int) Option {
	return func(cfg *Config) {
		cfg.TopWordsCount = topWordsCount
	}
}
