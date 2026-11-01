package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dua-os/dua-os-be/internal/bootstrap"
	"github.com/dua-os/dua-os-be/internal/platform/config"
	"github.com/dua-os/dua-os-be/internal/platform/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel, cfg.LogFormat)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	app, err := bootstrap.New(ctx, cfg, log)
	cancel()
	if err != nil {
		log.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	defer app.Close()

	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "version", cfg.Version)
		if err := app.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdown, done := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer done()
	log.Info("shutting down")
	if err := app.Server.Shutdown(shutdown); err != nil {
		log.Error("shutdown failed", "error", err)
		os.Exit(1)
	}
}
