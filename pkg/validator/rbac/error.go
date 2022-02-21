package rbac

import "fmt"

// Error is a custom rbac error type
type Error interface {
	error
	OrigError() error
}

type rbacError struct {
	message string
}

func (r *rbacError) OrigError() error {
	return fmt.Errorf(r.message)
}

func (r *rbacError) Error() string {
	return fmt.Sprintf("failed to validate rbac: %s", r.message)
}

// NewRBACError is a constructor for rbac error
func NewRBACError(message string) Error {
	return &rbacError{
		message: message,
	}
}
