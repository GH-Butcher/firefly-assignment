package main

import (
	"context"
	"encoding/json"
	"firefly-assignment/pkg/assays"
	"firefly-assignment/pkg/logger"
	"firefly-assignment/pkg/wordsbank"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	urlsListPath         = "assays.list"
	wordsBankUrl         = "https://raw.githubusercontent.com/dwyl/english-words/master/words.txt"
	contextTimeout       = 5 * time.Minute
	minWordLength        = 3
	maxUrlsToFetch       = 1000
	bufferSize           = 128
	ratePerSecond        = 30
	workers              = 20
	desiredTopWordsCount = 10
	defaultTimeout       = 30 * time.Second
	shardsCount          = 32 //--> Should be power of 2 to use fast modulo
)

func main() {
	// Set up context with graceful shutdown on SIGINT (Ctrl+C) or SIGTERM (docker/k8s stop)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log := logger.Init(logger.WithLogLevel(slog.LevelDebug)).Log

	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	bankCfg := wordsbank.NewConfig(
		wordsbank.WithLog(log),
		wordsbank.WithMinWordLength(minWordLength),
		wordsbank.WithUrl(wordsBankUrl),
		wordsbank.WithHttpClient(httpClient),
	)
	wbank, err := bankCfg.LoadBankOfWords(ctx)
	if err != nil {
		log.Error("Bank of words loading error: ", err, "")
		return
	}
	log.Info("Bank of words loaded", "words count", len(wbank))

	//---> assays service config
	srvConfig := assays.NewConfig(
		assays.WithBuffer(bufferSize),
		assays.WithMinWordLength(minWordLength),
		assays.WithMaxUrlsToFetch(maxUrlsToFetch),
		assays.WithRatePerSecond(ratePerSecond),
		assays.WithWorkers(workers),
		assays.WithUrlsListPath(urlsListPath),
		assays.WithTopWordsCount(desiredTopWordsCount),
		assays.WithShardsCount(shardsCount),
	)
	//---> assays service
	assayHandler := assays.NewService(log, srvConfig, wbank, httpClient)

	resultsChan := make(chan assays.Result, 1)
	errorsChan := make(chan error, 1)

	go func() {
		res, aErr := assayHandler.HandleAssays(ctx)
		if aErr != nil {
			errorsChan <- aErr
			return
		}
		resultsChan <- res
	}()

	select {
	case <-ctx.Done():
		log.Error("Context cancelled")
		select {
		case res := <-resultsChan:
			jsonOutputHelper(res, log)
			log.Info("Shutdown complete with partial/final results")
			if len(res.Errors) > 0 {
				log.Warn("Completed with partial failures",
					"succeeded", res.AssaysResult.HandledAssaysCount,
					"failed", len(res.Errors),
					"total", res.AssaysResult.TargetAssaysCount)
				for _, err := range res.Errors {
					log.Error("Error received", "error", err, "url", err.Url)
				}
			}
		case err = <-errorsChan:
			log.Error("Error received", "error", err)
		case <-time.After(300 * time.Millisecond):
			log.Info("No partial/final results received, shutting down..")
		}
	case err = <-errorsChan:
		log.Error("Error received", "error", err)
		return
	case res := <-resultsChan:
		jsonOutputHelper(res, log)
		if len(res.Errors) > 0 {
			log.Warn("Completed with partial failures",
				"succeeded", res.AssaysResult.HandledAssaysCount,
				"failed", len(res.Errors),
				"total", res.AssaysResult.TargetAssaysCount)
			for _, err := range res.Errors {
				log.Error("Error received", "error", err, "url", err.Url)
			}
		}
	}
}

func jsonOutputHelper(res assays.Result, log *slog.Logger) {
	jo, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		log.Error("Error marshaling JSON", "error", err)
		return
	}
	_, err = os.Stdout.Write(jo)
	if err != nil {
		log.Error("Error writing to stdout", "error", err)
		return
	}
	_, err = os.Stdout.Write([]byte("\n"))
	if err != nil {
		log.Error("Error writing to stdout", "error", err)
		return
	}
}
