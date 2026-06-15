package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/akhmed9505/event-booker/internal/components"
	"github.com/akhmed9505/event-booker/internal/config"

	"github.com/wb-go/wbf/zlog"
	"golang.org/x/sync/errgroup"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}

	// Инициализация глобального логгера zlog
	zlog.Init()

	// Берём глобальный логгер
	logger := zlog.Logger

	appCtx, cancel := context.WithCancel(context.Background())
	eg, erctx := errgroup.WithContext(appCtx)

	components, err := components.InitComponents(erctx, cfg, logger)
	if err != nil {
		logger.Error().Err(err).Msg("Could not init components")
		os.Exit(1)
	}

	eg.Go(func() error {
		if err := components.HttpServer.Run(erctx); err != nil {
			logger.Error().Err(err).Msg("The http Server failed")
			return err
		}
		logger.Info().Msg("Http Server stopped")
		return nil
	})

	quitChan := make(chan os.Signal, 1)
	signal.Notify(quitChan, os.Interrupt, syscall.SIGTERM)
	sig := <-quitChan

	cancel()

	logger.Info().Str("signal", sig.String()).Msg("Captured signal, initiating shutdown")

	if err = eg.Wait(); err != nil {
		logger.Error().Err(err).Msg("Application finished with error")
		if shutdownErr := components.ShutdownAll(); shutdownErr != nil {
			logger.Error().Err(shutdownErr).Msg("Error during component shutdown after errgroup error")
		}
		os.Exit(1)
	} else {
		logger.Info().Msg("Shutting down the services...")
		if shutdownErr := components.ShutdownAll(); shutdownErr != nil {
			logger.Error().Err(shutdownErr).Msg("could not properly stop manager server")
			os.Exit(1)
		}
	}

	logger.Info().Msg("All components stopped. Giving system a moment to flush logs...")
	logger.Info().Msg("Gracefully shutting down the servers")
}
