package serialize

import (
	"errors"
)

var (
	ErrMarshal   = errors.New("failed to marshal data")
	ErrUnmarshal = errors.New("failed to unmarshal data")
)
