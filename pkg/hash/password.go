package hash

import (
	"crypto/sha1"
	"fmt"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type SHa1Hasher struct {
	salt string
}

func NewSHa1Hasher(salt string) (*SHa1Hasher, error) {
	if salt == "" {
		return nil, fmt.Errorf("empty salt for SHa1Hasher")
	}

	return &SHa1Hasher{salt: salt}, nil
}

func (h *SHa1Hasher) Hash(password string) (string, error) {
	hash := sha1.New()

	if _, err := hash.Write([]byte(password)); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum([]byte(h.salt))), nil
}
