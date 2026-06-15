package storage

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/akhmed9505/event-booker/internal/domain"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (pg *Postgres) SaveUser(ctx context.Context, user *domain.User) (uuid.UUID, error) {
	rolesJSON, err := json.Marshal(user.Roles)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to marshal roles: %w", err)
	}

	row := pg.db.Master.QueryRowContext(ctx, "INSERT INTO users(nickname, password_hash, roles) VALUES ($1, $2, $3) RETURNING id", user.Nickname, user.PasswordHash, rolesJSON)
	//tokenRow := pg.pool.QueryRow(ctx, "INSERT INTO tokens(user_id, refresh_token, access_token, created_at, expires_at) VALUES ($1, $2, $3, $4, $5)")
	var id uuid.UUID
	if err := row.Scan(&id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName != "" {
			return uuid.Nil, domain.ErrNicknameAlreadyExist
		}

		return uuid.Nil, fmt.Errorf("storage.pg.SaveUser: %w", err)
	}

	return id, nil
}

func (pg *Postgres) GetUser(ctx context.Context, nickname string) (*domain.User, error) {
	row := pg.db.Master.QueryRowContext(ctx, "SELECT id, nickname, password_hash, roles, created_at, updated_at FROM users WHERE nickname = $1", nickname)
	var user domain.User

	var rolesJSON []byte
	err := row.Scan(&user.ID, &user.Nickname, &user.PasswordHash, &rolesJSON, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("storage.pg.GetUser: %w", err)
	}

	err = json.Unmarshal(rolesJSON, &user.Roles)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
