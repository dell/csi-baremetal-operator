package validator

import (
	"context"

	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

// rbacValidator is a private interface for rbac validator
type rbacValidator interface {
	ValidateServiceAccountIsBound(ctx context.Context, rules *models.ServiceAccountIsRoleBoundData) error
}
