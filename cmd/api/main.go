// @title Chimera API
// @version 1.0
// @description Music streaming backend
// @BasePath /
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chimera/internal/business_logic/usecase"
	"chimera/internal/config"
	"chimera/internal/controllers"
	"chimera/internal/infra/logger"

	_ "chimera/docs"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	log := logger.New(cfg)

	router := controllers.NewRouter(controllers.Dependencies{
		Users:  usecase.NewUserService(),
		Auth:   usecase.NewAuthService(),
		Tracks: usecase.NewTrackService(),
		Log:    log,
	})

	server := &http.Server{
		Addr:    cfg.HTTP.Addr,
		Handler: router,
	}

	log.Info("listening", "addr", cfg.HTTP.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	closeCh := make(chan os.Signal, 1)
	signal.Notify(closeCh, os.Interrupt, syscall.SIGTERM)

	<-closeCh
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("shutdown", "err", err)
		os.Exit(1)
	}

	log.Info("graceful shutdown")
}
