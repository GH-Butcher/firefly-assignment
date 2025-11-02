package assays

import (
	"firefly-assignment/pkg/models"
	"log/slog"
	"net/http"
	"time"
)

type Service struct {
	log        *slog.Logger
	cfg        *Config
	bank       map[string]struct{}
	httpClient *http.Client
}

type Result struct {
	Assays []models.Assay `json:"assays"`
}

type WordCount struct {
	Word  string `json:"word"`
	Count int    `json:"count"`
}

func NewService(log *slog.Logger, cfg *Config, bank map[string]struct{}) *Service {
	srv := &Service{
		log:  log,
		cfg:  cfg,
		bank: bank,
	}
	srv.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	return srv
}
