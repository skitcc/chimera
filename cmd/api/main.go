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
	"chimera/internal/infra/adapters/memory"
	"chimera/internal/infra/logger"

	_ "chimera/docs"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load("")
	if err != nil {
		slog.ErrorContext(ctx, "load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg)

	router := controllers.NewRouter(controllers.Dependencies{
		Users:  usecase.NewUserService(memory.NewUserRepository()),
		Auth:   usecase.NewAuthService(),
		Tracks: usecase.NewTrackService(memory.NewTrackRepository()),
		Log:    log,
	})

	server := &http.Server{
		Addr:    cfg.HTTP.Addr,
		Handler: router,
	}

	log.InfoContext(ctx, "listening", "addr", cfg.HTTP.Addr)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.ErrorContext(ctx, "http server", "error", err)
			os.Exit(1)
		}
	}()

	closeCh := make(chan os.Signal, 1)
	signal.Notify(closeCh, os.Interrupt, syscall.SIGTERM)

	<-closeCh
	shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.ErrorContext(ctx, "shutdown", "error", err)
		os.Exit(1)
	}

	log.InfoContext(ctx, "graceful shutdown")
}
