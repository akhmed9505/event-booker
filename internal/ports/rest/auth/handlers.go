package auth

import (
	"context"
	"errors"
	"github.com/akhmed9505/event-booker/internal/domain"
	"github.com/akhmed9505/event-booker/pkg/jwt"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/ginext"
)

//go:generate mockgen -source=handlers.go -destination=mocks/mock.go
type HandlerAuth interface {
	Register(ctx context.Context, nickname, password string, roles []jwt.Role) error
	Login(ctx context.Context, nickname, password string) (*domain.Tokens, *domain.User, error)
	Refresh(ctx context.Context, token string) (*domain.Tokens, error)
}

type Handler struct {
	logger zerolog.Logger
	auth   HandlerAuth
}

func NewHandler(logger zerolog.Logger, authService HandlerAuth) *Handler {
	return &Handler{
		logger: logger,
		auth:   authService,
	}
}

// Register регистрирует нового пользователя
// @Summary Регистрация пользователя
// @Description Регистрация нового пользователя по никнейму и паролю
// @Tags auth
// @Accept json
// @Produce json
// @Param input body domain.User true "Данные пользователя"
// @Success 200 {string} string "Успешная регистрация"
// @Failure 400 {string} string "Некорректные данные"
// @Failure 500 {string} string "Ошибка сервиса"
// @Router /auth/register [post]
func (h *Handler) Register(c *ginext.Context) {
	var register registerRequest

	if err := c.Bind(&register); err != nil {
		h.logger.Warn().Err(err).Msg("Failed to bind register request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	roleToAssign := register.Roles
	if len(roleToAssign) == 0 {
		roleToAssign = []jwt.Role{jwt.Viewer}
	}

	if err := validator.New().Struct(register); err != nil {
		h.logger.Warn().Err(err).Msg("Validation failed for register request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	err := h.auth.Register(c.Request.Context(), register.Nickname, register.Password, roleToAssign)
	if err != nil {
		if errors.Is(err, domain.ErrNicknameAlreadyExist) {
			h.logger.Info().Str("nickname", register.Nickname).Msg("Nickname already exists")
			c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
			return
		}
		h.logger.Error().Err(err).Msg("Failed to register user")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	h.logger.Info().Str("nickname", register.Nickname).Msg("User registered successfully")
	c.JSON(http.StatusOK, ginext.H{"Success": "success"})
}

// Login авторизует пользователя и возвращает токены
// @Summary Вход в систему
// @Description Авторизация пользователя по никнейму и паролю, получение JWT токенов
// @Tags auth
// @Accept json
// @Produce json
// @Param input body domain.User true "Данные для входа: никнейм и пароль"
// @Success 200 {object} domain.Tokens
// @Failure 401 {string} string "Неверные учетные данные"
// @Failure 500 {string} string "Ошибка сервиса"
// @Router /auth/login [post]
func (h *Handler) Login(c *ginext.Context) {
	var logReq loginRequest

	if err := c.Bind(&logReq); err != nil {
		h.logger.Warn().Err(err).Msg("Failed to bind login request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	if err := validator.New().Struct(logReq); err != nil {
		h.logger.Warn().Err(err).Msg("Validation failed for login request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	tokens, user, err := h.auth.Login(c.Request.Context(), logReq.Nickname, logReq.Password)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			h.logger.Info().Str("nickname", logReq.Nickname).Msg("Invalid credentials")
			c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
			return
		}
		h.logger.Error().Err(err).Msg("Failed to login user")
		c.JSON(http.StatusInternalServerError, ginext.H{"error": err.Error()})
		return
	}

	tokenResp := tokenResponse{
		UserID:       user.ID,
		Nickname:     user.Nickname,
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		CreatedAt:    tokens.CreatedAt,
		ExpiresAt:    tokens.ExpiresAt,
		Roles:        user.Roles,
	}

	c.SetCookie(
		"jwt_token",
		tokens.AccessToken,
		3600*24, // cookie lifetime in seconds
		"/",
		"localhost",
		false, // secure, true if HTTPS
		true,  // httpOnly
	)

	h.logger.Info().Str("nickname", user.Nickname).Str("userID", user.ID.String()).Msg("User logged in successfully")

	c.JSON(http.StatusOK, tokenResp)
}

// Refresh обновляет токены по refresh токену
// @Summary Обновление токенов
// @Description Обновляет JWT и refresh токены по действующему refresh токену
// @Tags auth
// @Accept json
// @Produce json
// @Param token body string true "Refresh токен"
// @Success 200 {object} domain.Tokens
// @Failure 401 {string} string "Некорректный refresh токен"
// @Failure 500 {string} string "Ошибка при обновлении токенов"
// @Router /auth/refresh [post]
func (h *Handler) RefreshToken(c *ginext.Context) {
	var refresh refreshRequest

	if err := c.ShouldBindJSON(&refresh); err != nil { // Используем ShouldBindJSON
		h.logger.Warn().Err(err).Msg("Failed to bind refresh token request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	if err := validator.New().Struct(refresh); err != nil {
		h.logger.Warn().Err(err).Msg("Validation failed for refresh token request")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	tokens, err := h.auth.Refresh(c.Request.Context(), refresh.RefreshToken)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			h.logger.Info().Msg("Refresh failed: user not found")
			c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
			return
		}
		h.logger.Error().Err(err).Msg("Failed to refresh tokens")
		c.JSON(http.StatusBadRequest, ginext.H{"error": err.Error()})
		return
	}

	tokenResp := tokenResponseRefresh{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		CreateAt:     tokens.CreatedAt,
		ExpiresAt:    tokens.ExpiresAt,
	}

	h.logger.Info().Msg("Tokens refreshed successfully")

	c.JSON(http.StatusOK, ginext.H{"success": tokenResp})
}
