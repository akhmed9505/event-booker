package auth

import (
	"github.com/akhmed9505/event-booker/pkg/jwt"
	"time"

	"github.com/google/uuid"
)

type registerRequest struct {
	Nickname string     `json:"nickname" validate:"required"`
	Password string     `json:"password" validate:"required"`
	Roles    []jwt.Role `json:"roles"`
}

// Запрос логина пользователя
type loginRequest struct {
	Nickname string `json:"nickname" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// Ответ с токенами после аутентификации
type tokenResponse struct {
	UserID       uuid.UUID  `json:"userID"`
	Nickname     string     `json:"nickname"`
	AccessToken  string     `json:"token"` // изменено с "AccessToken" на "token"
	RefreshToken string     `json:"refresh_token"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	Roles        []jwt.Role `json:"roles"`
}

type tokenResponseRefresh struct {
	AccessToken  string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	CreateAt     time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Запрос обновления токена
type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
