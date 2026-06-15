package ports

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/akhmed9505/event-booker/internal/config"
	"github.com/akhmed9505/event-booker/internal/ports/rest/auth"
	eventbroker "github.com/akhmed9505/event-booker/internal/ports/rest/eventBroker"
	"github.com/akhmed9505/event-booker/internal/ports/rest/renderer"
	"github.com/akhmed9505/event-booker/pkg/jwt"

	"github.com/gin-contrib/cors"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/ginext"
)

var (
	RoleUser  jwt.Role = "user"
	RoleAdmin jwt.Role = "admin"
)

type Server struct {
	logger zerolog.Logger
	server *ginext.Engine
	cfg    config.Config
}

func NewServer(cfg *config.Config, logger zerolog.Logger, authHandler auth.HandlerAuth, rend renderer.RenderHandler, eventHandler eventbroker.EventHandler, bookHandler eventbroker.BookingHandler, manager jwt.TokenManager, notifier eventbroker.Notifier) *Server {
	handler := auth.NewHandler(logger, authHandler)
	broker := eventbroker.NewHandler(cfg, logger, eventHandler, bookHandler, notifier)
	render := renderer.NewHandler(rend)
	r := InitRouter(render, broker, handler, manager)
	return &Server{
		server: r,
		cfg:    *cfg,
	}
}

func InitRouter(rend *renderer.Handler, h *eventbroker.Handler, auth *auth.Handler, tokenManager jwt.TokenManager) *ginext.Engine {
	r := ginext.New()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:8080"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD"}
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	config.AllowCredentials = true
	r.Use(cors.New(config))
	r.Use(ginext.Logger())
	r.Use(ginext.Recovery())

	r.LoadHTMLGlob("templates/*")
	r.GET("/login", rend.Loginpage)
	r.GET("/register", rend.Registerpage)

	r.POST("/user/register", auth.Register)
	r.POST("/user/login", auth.Login)
	r.POST("/user/refresh", auth.RefreshToken)

	// Группа API с JWT Middleware
	api := r.Group("/api")
	api.Use(jwt.ValidateTokenMiddleware(tokenManager))
	// Применение RequireRole для ролей в API-группе
	api.POST("/events", jwt.RequireRole(RoleAdmin), h.Create)
	api.GET("/events/:id", jwt.RequireRole(RoleUser, RoleAdmin), h.Get)

	// booking — бронирование и управление для авторизованных пользователей
	api.POST("/events/:id/book", jwt.RequireRole(RoleUser, RoleAdmin), h.BookSeat)
	api.POST("/bookings/:booking_id/confirm", jwt.RequireRole(RoleUser, RoleAdmin), h.Confirm)
	api.POST("/bookings/:booking_id/cancel", jwt.RequireRole(RoleUser, RoleAdmin), h.Cancel)
	// HTML роуты (отдача страниц) — тоже с авторизацией, можно так:
	r.Use(jwt.ValidateTokenMiddleware(tokenManager))
	r.GET("/events/:id/bookings", jwt.RequireRole(RoleAdmin), h.ShowBookingList) // список броней — только адми
	r.GET("/events/:id/book", jwt.RequireRole(RoleUser, RoleAdmin), h.ShowBookingForm)
	r.GET("/events", jwt.RequireRole(RoleUser, RoleAdmin), h.ShowEventList)
	r.GET("/events/new", jwt.RequireRole(RoleAdmin), h.ShowEvent)
	r.GET("/events/:id/view", jwt.RequireRole(RoleUser, RoleAdmin), h.ShowEventDetail)
	r.GET("/profile", h.ShowProfile)
	r.POST("/profile", h.UpdateProfile)

	return r
}

func (s *Server) Run(ctx context.Context) error {
	errChan := make(chan error, 1)
	srv := &http.Server{
		Addr:    ":" + s.cfg.Http.Port,
		Handler: s.server,
	}
	go func() {
		s.logger.Info().Str("port", s.cfg.Http.Port).Msg("Starting listening")
		err := srv.ListenAndServe()

		if err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("Server failed to start")
			errChan <- fmt.Errorf("ListenAndServe error: %w", err)
		} else {
			errChan <- nil
		}
	}()

	select {
	case <-ctx.Done():
		s.logger.Info().Str("reason", ctx.Err().Error()).Msg("Shutting down the server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			s.logger.Error().Err(err).Msg("Server forced to shutdown")
			return err
		}
		s.logger.Info().Msg("Server stopped gracefully")
		return nil
	case err := <-errChan:
		if err != nil {
			s.logger.Error().Err(err).Msg("HttpServer failed")
			return err
		}
		s.logger.Info().Msg("HttpServer stopped gracefully (without external signal).")
		return nil
	}
}
