package configstore

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound               = errors.New("not found")
	ErrAlreadyExists          = errors.New("already exists")
	ErrConfigRevisionConflict = errors.New("config revision conflict")
)

// ConfigRevisionConflictError reports a stale configuration mutation.
type ConfigRevisionConflictError struct {
	Expected int64
	Actual   int64
}

func (e *ConfigRevisionConflictError) Error() string {
	return fmt.Sprintf("%v: expected %d, current %d", ErrConfigRevisionConflict, e.Expected, e.Actual)
}

// Unwrap supports errors.Is with ErrConfigRevisionConflict.
func (e *ConfigRevisionConflictError) Unwrap() error {
	return ErrConfigRevisionConflict
}

// ErrUnresolvedKeys is returned when one or more keys could not be resolved
type ErrUnresolvedKeys struct {
	Identifiers []string
}

func (e *ErrUnresolvedKeys) Error() string {
	return fmt.Sprintf("could not resolve keys: %s", strings.Join(e.Identifiers, ", "))
}
