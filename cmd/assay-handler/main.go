package main

import (
	"context"
	"encoding/json"
	"firefly-assignment/pkg/assays"
	"firefly-assignment/pkg/logger"
	"firefly-assignment/pkg/words_bank"
	"log/slog"
	"os"
	"os/signal"
)

func main() {
	bankConfig := words_bank.NewConfig()
	assayConfig := assays.NewConfig(assays.WithMax(100))

	log := logger.Init(logger.WithLevel(slog.LevelInfo)).Log

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	log.Info("Loading Bank of Words")

	bank, err := words_bank.LoadWordBank(ctx, bankConfig)
	if err != nil {
		log.Error("Failed to load bank of words", "error", err)
		return
	}

	log.Info("Bank of Words Loaded", "words", len(bank))

	//--> Start Service
	// Run processing in a goroutine so we can react to signals
	resultCh := make(chan assays.Result, 1)
	errCh := make(chan error, 1)

	go func() {
		res, e := assays.ProcessTopWords(ctx, log, assayConfig, bank)
		if e != nil {
			errCh <- e
			return
		}
		resultCh <- res
	}()

	select {
	case <-ctx.Done():
		// Signal received, cancel propagated; workers will stop gracefully.
		log.Info("Shutdown requested, waiting for workers to finish...")
		// A second select to wait briefly for a final result if it arrives quickly.
		select {
		case res := <-resultCh:
			out, _ := json.MarshalIndent(res, "", "  ")
			os.Stdout.Write(out)
			os.Stdout.Write([]byte("\n"))
			log.Info("Shutdown complete with partial/final results")
		case <-errCh:
			log.Info("Shutdown complete with errors")
		default:
		}
	case err = <-errCh:
		log.Error("Failed processing essays", "error", err)
		os.Exit(1)
	case res := <-resultCh:
		out, _ := json.MarshalIndent(res, "", "  ")
		os.Stdout.Write(out)
		os.Stdout.Write([]byte("\n"))
		log.Info("Processing completed")
	}
}
