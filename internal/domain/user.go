package domain

import (
	"github.com/akhmed9505/event-booker/pkg/jwt"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Nickname     string     `json:"nickname"`
	Email        string     `json:"email"`
	TelegramID   string     `json:"telegram_id"`
	PasswordHash string     `json:"password_hash"`
	Roles        []jwt.Role `json:"roles"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Tokens struct {
	ID           uuid.UUID // bigint (bigserial) в базе
	UserID       uuid.UUID `json:"user_id"`
	AccessToken  string    `json:"access-token"`
	RefreshToken string    `json:"refresh-token"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type UserClaims struct {
	UserID   uuid.UUID  `json:"user_id"` // bigint
	Nickname string     `json:"nickname"`
	Roles    []jwt.Role `json:"roles"`
}

type UserInfo struct {
	UserID   uuid.UUID
	Nickname string
	Roles    []jwt.Role
}
