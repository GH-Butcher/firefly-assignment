package assays

type Option func(*Config)
type Config struct {
	MinWordLength int
	Workers       int
	RatePerSecond int
	Buffer        int
	ListPath      string
	Max           int
}

func NewConfig(opts ...Option) *Config {
	c := &Config{
		MinWordLength: 3,
		Max:           100,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func WithMinWordLength(minWordLength int) Option {
	return func(c *Config) {
		c.MinWordLength = minWordLength
	}
}

func WithMax(max int) Option {
	return func(c *Config) {
		c.Max = max
	}
}

func WithWorkers(workers int) Option {
	return func(c *Config) {
		c.Workers = workers
	}
}

func WithRatePerSecond(ratePerSecond int) Option {
	return func(c *Config) {
		c.RatePerSecond = ratePerSecond
	}
}

func WithBuffer(buffer int) Option {
	return func(c *Config) {
		c.Buffer = buffer
	}
}
