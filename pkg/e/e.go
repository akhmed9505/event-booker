package e

import (
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("order not found")

func Wrap(message string, err error) error {
	return fmt.Errorf("%s: %w", message, err)
}
