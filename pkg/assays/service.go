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
	TargetAssaysCount    int         `json:"target_assays_count"`
	HandledAssaysCount   int64       `json:"handled_assays_count"`
	DesiredTopWordsCount int         `json:"desired_top_words_count"`
	FailedAssaysCount    int         `json:"failed_assays_count"`
	DesiredWordLength    int         `json:"desired_word_length"`
	TopWords             []WordCount `json:"top_words"`
}

type Result struct {
	AssaysResult AssayResult `json:"assays_result"`
	Errors       []URLError  `json:"errors,omitempty"`
}

type URLError struct {
	Url string `json:"url"`
	Err string `json:"error"`
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
