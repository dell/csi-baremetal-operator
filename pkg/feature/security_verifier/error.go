package securityverifier

import "fmt"

// Error is a custom rbac error type
type Error interface {
	error
}

type verifierError struct {
	message string
}

func (r *verifierError) Error() string {
	return fmt.Sprintf("failed to verify: %s", r.message)
}

// NewVerifierError is a constructor for rbac error
func NewVerifierError(message string) Error {
	return &verifierError{
		message: message,
	}
}
