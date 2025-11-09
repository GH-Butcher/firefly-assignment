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
	"time"
)

const (
	urlsListPath   = "assays.list"
	contextTimeout = 10 * time.Second
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	log := logger.Init(logger.WithLogLevel(slog.LevelDebug)).Log

	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	bankCfg := wordsbank.NewConfig(
		wordsbank.WithLog(log),
		wordsbank.WithMinWordLength(3),
		wordsbank.WithUrl("https://raw.githubusercontent.com/dwyl/english-words/master/words.txt"),
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
		assays.WithBuffer(128),
		assays.WithMinWordLength(3),
		assays.WithMaxUrlsToFetch(500),
		assays.WithRatePerSecond(10),
		assays.WithWorkers(10),
		assays.WithUrlsListPath(urlsListPath),
		assays.WithTopWordsCount(10),
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
			jsonOutput, _ := json.MarshalIndent(res, "", "  ")
			os.Stdout.Write(jsonOutput)
			os.Stdout.Write([]byte("\n"))
			log.Info("Shutdown complete with partial/final results")
		case err = <-errorsChan:
			log.Error("Error received", "error", err)
		case <-time.After(300 * time.Millisecond):
			log.Info("No partial/final results received, shutting down..")
		}
	case err = <-errorsChan:
		log.Error("Error received", "error", err)
		return
	case res := <-resultsChan:
		jsonOutput, _ := json.MarshalIndent(res, "", "  ")
		os.Stdout.Write(jsonOutput)
		os.Stdout.Write([]byte("\n"))
		log.Info("Proccessing complete..")
	}
}
