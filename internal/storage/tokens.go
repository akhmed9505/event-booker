package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/akhmed9505/event-booker/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Token interface {
	StoreRefreshToken(ctx context.Context, userID uuid.UUID, refreshToken string, expiresAt time.Time) error
	GetUserByRefreshToken(ctx context.Context, refreshToken string) (*domain.User, error)
}

// HashToken создает SHA256 хеш токена
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (pg *Postgres) StoreRefreshToken(ctx context.Context, userID uuid.UUID, refreshToken string, expiresAt time.Time) error {

	_, err := pg.db.Master.ExecContext(ctx, `
        INSERT INTO tokens (user_id, refresh_token,expires_at)
        VALUES ($1, $2, $3)
        ON CONFLICT (user_id) DO UPDATE
        SET refresh_token = EXCLUDED.refresh_token,
            expires_at = EXCLUDED.expires_at
    `, userID, refreshToken, expiresAt)
	if err != nil {
		return fmt.Errorf("storage.pg.StoreRefreshToken: %w", err)
	}
	return nil
}

func (pg *Postgres) GetUserByRefreshToken(ctx context.Context, refreshToken string) (*domain.User, error) {
	row := pg.db.Master.QueryRowContext(ctx, `
        SELECT u.id, u.nickname, u.roles
        FROM tokens rt
        JOIN users u ON u.id = rt.user_id
        WHERE rt.refresh_token = $1 AND rt.expires_at > NOW()
    `, refreshToken)

	var user domain.User
	var rolesJSON []byte
	err := row.Scan(&user.ID, &user.Nickname, &rolesJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("storage.pg.GetUserByRefreshToken: %w", err)
	}

	// Распарсим JSONB поле roles в срез Role
	if err := json.Unmarshal(rolesJSON, &user.Roles); err != nil {
		return nil, fmt.Errorf("storage.pg.GetUserByRefreshToken: failed to unmarshal roles: %w", err)
	}

	return &user, nil
}
