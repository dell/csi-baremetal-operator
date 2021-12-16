package rbac

import "fmt"

// Error ...
type Error interface {
	error
}

type rbacError struct {
	message string
}

func (r *rbacError) Error() string {
	return fmt.Sprintf("failed to validate rbac: %s", r.message)
}

// NewRBACError ...
func NewRBACError(message string) Error {
	return &rbacError{
		message: message,
	}
}
