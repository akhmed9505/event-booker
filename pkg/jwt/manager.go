package jwt

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type TokenManager interface {
	NewJWT(userID uuid.UUID, nickname string, roles []Role, ttl time.Duration) (string, error)
	Parse(accessToken string) (*UserInfo, error)
	NewRefreshToken() (string, error)
}

type UserInfo struct {
	UserID   uuid.UUID
	Nickname string
	Roles    []Role
}

type CustomClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	Nickname string    `json:"nickname"`
	Roles    []string  `json:"roles"`
	jwt.RegisteredClaims
}

type JWTManager struct{ signingKey string }

type Role string
type Permission string

const (
	Admin   Role = "admin"
	Manager Role = "manager"
	Viewer  Role = "viewer"

	Read   Permission = "read"
	Write  Permission = "write"
	Delete Permission = "delete"
	Update Permission = "update"
)

func NewManager(signingKey string) (*JWTManager, error) {
	if signingKey == "" {
		return nil, fmt.Errorf("empty signing key")
	}
	return &JWTManager{signingKey: signingKey}, nil
}

func (m *JWTManager) NewJWT(userID uuid.UUID, nickname string, roles []Role, ttl time.Duration) (string, error) {

	rolesStr := make([]string, len(roles))
	for i, r := range roles {
		rolesStr[i] = string(r)
	}

	claims := &CustomClaims{
		UserID:   userID,
		Nickname: nickname,
		Roles:    rolesStr,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(m.signingKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}
	return tokenString, nil
}

func (m *JWTManager) Parse(accessToken string) (*UserInfo, error) {
	claims := &CustomClaims{}

	token, err := jwt.ParseWithClaims(accessToken, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(m.signingKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	roles := make([]Role, len(claims.Roles))
	for i, r := range claims.Roles {
		roles[i] = Role(r)
	}

	return &UserInfo{
		UserID:   claims.UserID,
		Nickname: claims.Nickname,
		Roles:    roles,
	}, nil
}

func (m *JWTManager) NewRefreshToken() (string, error) {
	b := make([]byte, 32)

	s := rand.NewSource(time.Now().Unix())
	r := rand.New(s)

	if _, err := r.Read(b); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", b), nil
}
