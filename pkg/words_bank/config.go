package words_bank

import (
	"net/http"
	"time"
)

type Option func(*Config)
type Config struct {
	URL            string
	DefaultTimeout time.Duration
	Client         *http.Client

	MinWordLength int
}

func NewConfig(opts ...Option) *Config {
	c := &Config{
		URL:            "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt",
		DefaultTimeout: 10 * time.Second,
		Client: &http.Client{
			Timeout: 10 * time.Second,
		},
		MinWordLength: 3,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Ensure client timeout aligns with DefaultTimeout unless the client was overridden after timeout was set.
	if c.Client != nil && c.Client.Timeout == 0 && c.DefaultTimeout > 0 {
		c.Client.Timeout = c.DefaultTimeout
	}
	return c
}

func WithUrl(url string) Option {
	return func(c *Config) {
		c.URL = url
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.DefaultTimeout = timeout
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.Client = client
		if c.Client != nil && c.Client.Timeout == 0 && c.DefaultTimeout > 0 {
			c.Client.Timeout = c.DefaultTimeout
		}
	}
}
