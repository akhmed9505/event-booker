package components

import (
	"context"
	"github.com/akhmed9505/event-booker/internal/config"
	"github.com/akhmed9505/event-booker/internal/ports"
	"github.com/akhmed9505/event-booker/internal/service/auth"
	"github.com/akhmed9505/event-booker/internal/service/events"
	"github.com/akhmed9505/event-booker/internal/service/render"
	"github.com/akhmed9505/event-booker/internal/storage"
	"github.com/akhmed9505/event-booker/pkg/jwt"
	"fmt"
	"os"

	"github.com/rs/zerolog"
)

type Components struct {
	Logger     zerolog.Logger
	HttpServer *ports.Server
	Postgres   *storage.Postgres
}

func InitComponents(ctx context.Context, cfg *config.Config, logger zerolog.Logger) (*Components, error) {
	// Инициализация базы данных
	logger.Info().Msg("Initializing Postgres...")
	postgres, err := storage.NewPostgres(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to init postgres: %w", err)
	}
	logger.Info().Msg("Initialized Postgres")
	// Текущая директория для шаблонов
	cwd, err := os.Getwd()
	if err != nil {
		logger.Error().Err(err).Msg("failed to get current directory")
		return nil, err
	}
	logger.Info().Str("cwd", cwd).Msg("Current working directory")

	// Инициализация шаблонизатора
	renderSrv := render.New(cwd+"/templates", logger)

	// Инициализация аутентификации
	logger.Info().Msg("Initializing auth...")
	authSrv, err := auth.NewAuth(cfg, postgres, postgres)
	if err != nil {
		return nil, fmt.Errorf("failed to init auth service: %w", err)
	}
	logger.Info().Msg("Initialized auth")

	// Инициализация службы событий и бронирований
	eventBroker := events.NewService(postgres, postgres, nil)

	// Запуск фонового cron очистки просроченных броней
	go func() error {
		if err := eventBroker.StartExpiredBookingCleanerCron(ctx, "@every 1m"); err != nil {
			return fmt.Errorf("failed to start expired booking cleaner cron: %w", err)
		}
		return nil
	}()

	tokenManager, err := jwt.NewManager(cfg.AuthConfig.JWTSigningKey)
	if err != nil {
		return nil, err
	}
	logger.Info().Msg("Initializing server...")
	// Инициализация HTTP сервера с внедрением зависимостей

	httpServer := ports.NewServer(cfg, logger, authSrv, renderSrv, eventBroker, eventBroker, tokenManager, eventBroker)
	logger.Info().Msg("Initialized server")
	return &Components{
		Logger:     logger,
		HttpServer: httpServer,
		Postgres:   postgres,
	}, nil
}

func (c *Components) ShutdownAll() error {
	c.Logger.Info().Msg("ShutdownAll: starting...")

	if err := c.Postgres.Close(); err != nil {
		c.Logger.Error().Err(err).Msg("Error closing Postgres connection")
	}

	c.Logger.Info().Msg("ShutdownAll: finished")
	return nil
}
