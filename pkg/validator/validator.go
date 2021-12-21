package validator

import (
	"context"
	"fmt"

	"github.com/dell/csi-baremetal-operator/pkg/validator/models"
	rbacmodels "github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

// Validator is a generic validator for validating certain conditions (e.g. rbac resources matches)
type Validator interface {
	ValidateRBAC(ctx context.Context, rules *models.RBACRules) error
}

type validator struct {
	rbacValidator
}

func (v *validator) ValidateRBAC(ctx context.Context, rules *models.RBACRules) (err error) {
	switch rules.Type {
	case models.ServiceAccountIsRoleBound:
		adaptedRules, ok := rules.Data.(*rbacmodels.ServiceAccountIsRoleBoundData)
		if !ok {
			return fmt.Errorf("unknown data for service account is role bound validation")
		}
		return v.ValidateServiceAccountIsBound(ctx, adaptedRules)
	default:
		return fmt.Errorf("unknown validation rule type, %s", rules.Type)
	}
}

// NewValidator is a constructor for validator
func NewValidator(rbacValidator rbacValidator) Validator {
	return &validator{
		rbacValidator: rbacValidator,
	}
}
