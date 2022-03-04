package securityverifier

import "fmt"

// Error is a custom security verifier error type
type Error interface {
	error
	OrigError() error
}

type verifierError struct {
	message string
}

func (r *verifierError) OrigError() error {
	return fmt.Errorf(r.message)
}

func (r *verifierError) Error() string {
	return fmt.Sprintf("failed to verify: %s", r.message)
}

// NewVerifierError is a constructor for security verifier error
func NewVerifierError(message string) Error {
	return &verifierError{
		message: message,
	}
}
