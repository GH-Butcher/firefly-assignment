package assays

import (
	"log/slog"
	"net/http"
	"time"
)

type Service struct {
	log        *slog.Logger
	config     *Config
	wordsBank  map[string]struct{}
	httpClient *http.Client
}

type AssayResult struct {
	AssaysCount          int         `json:"assays_count"`
	DesiredTopWordsCount int         `json:"desired_top_words_count"`
	TopWords             []WordCount `json:"top_words"`
}

type Result struct {
	AssaysResult AssayResult `json:"assays_result"`
}

type WordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

func NewService(log *slog.Logger, config *Config, bank map[string]struct{}, client *http.Client) *Service {
	srv := &Service{
		log:       log,
		config:    config,
		wordsBank: bank,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	if client != nil {
		srv.httpClient = client
	}
	return srv
}
