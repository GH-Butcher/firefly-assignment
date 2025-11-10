package assays

const (
	minWordLength  = 3
	maxUrlsToFetch = 0
	workers        = 10
	buffer         = 128
	ratePerSecond  = 30
	topWordsCount  = 10
	shardsCount    = 10
)

type Option func(*Config)

type Config struct {
	MinWordLength  int
	MaxUrlsToFetch int
	UrlsListPath   string
	Workers        int
	Buffer         int
	RatePerSecond  int
	TopWordsCount  int
	ShardsCount    int
}

func NewConfig(opts ...Option) *Config {
	cfg := &Config{
		MinWordLength:  minWordLength,
		MaxUrlsToFetch: maxUrlsToFetch,
		Workers:        workers,
		Buffer:         buffer,
		RatePerSecond:  ratePerSecond,
		TopWordsCount:  topWordsCount,
		ShardsCount:    shardsCount,
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

func WithShardsCount(shardsCount int) Option {
	return func(cfg *Config) {
		cfg.ShardsCount = shardsCount
	}
}
