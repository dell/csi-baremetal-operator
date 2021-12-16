package validator

import (
	"context"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

// rbacValidator ...
type rbacValidator interface {
	ValidateServiceAccountIsBound(ctx context.Context, rules *models.ServiceAccountIsRoleBoundData) error
}
