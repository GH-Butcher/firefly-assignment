package assays

type Option func(*Config)
type Config struct {
	MinWordLength int
	Workers       int
	RatePerSecond int
	Buffer        int
	ListPath      string
	Max           int
	StripHTML     bool
}

func NewConfig(opts ...Option) *Config {
	c := &Config{
		MinWordLength: 4,
		Max:           100,
		StripHTML:     true,
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

func WithListPath(listPath string) Option {
	return func(c *Config) {
		c.ListPath = listPath
	}
}

func WithStripHTML(stripHTML bool) Option {
	return func(c *Config) {
		c.StripHTML = stripHTML
	}
}
