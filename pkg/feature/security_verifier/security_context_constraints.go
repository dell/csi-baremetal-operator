package securityverifier

import (
	"context"
	"errors"
	"fmt"

	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	verifierModels "github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier/models"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	validatorModels "github.com/dell/csi-baremetal-operator/pkg/validator/models"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
	rbacModels "github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

type securityContextConstraintsVerifier struct {
	validator     validator.Validator
	eventRecorder events.EventRecorder
	matchPolicies []rbacv1.PolicyRule
	log           *logrus.Entry
}

func (v *securityContextConstraintsVerifier) Verify(ctx context.Context, csi *csibaremetalv1.Deployment, component verifierModels.Component) error {
	var serviceAccount string
	switch component {
	case verifierModels.Node:
		serviceAccount = csi.Spec.Driver.Node.ServiceAccount
	case verifierModels.Scheduler:
		serviceAccount = csi.Spec.Scheduler.ServiceAccount
	default:
		return fmt.Errorf("unknown component was passed")
	}

	return v.validator.ValidateRBAC(ctx, &validatorModels.RBACRules{
		Data: &rbacModels.ServiceAccountIsRoleBoundData{
			ServiceAccountName: serviceAccount,
			Namespace:          csi.Namespace,
			Role: &rbacv1.Role{
				Rules: v.matchPolicies,
			},
		},
		Type: validatorModels.ServiceAccountIsRoleBound,
	})
}

func (v *securityContextConstraintsVerifier) HandleError(_ context.Context, csi *csibaremetalv1.Deployment, serviceAccount string, err error) error {
	var rbacError rbac.Error
	if errors.As(err, &rbacError) {
		v.eventRecorder.Eventf(csi, eventing.WarningType, "SecurityContextConstraintsVerificationFailed",
			"ServiceAccount %s has insufficient securityContextConstraints, should have privileged",
			serviceAccount)
		v.log.Warning(rbacError, "Service account has insufficient securityContextConstraints, should have privileged")
		return NewVerifierError("Service account has insufficient securityContextConstraints, should have privileged")
	}
	v.log.Error(err, "Error occurred while validating service account security context bindings")
	return err
}

// NewSecurityContextConstraintsVerifier is a constructor for security context constraints verifier
func NewSecurityContextConstraintsVerifier(
	validator validator.Validator,
	eventRecorder events.EventRecorder,
	matchPolicies []rbacv1.PolicyRule,
	log *logrus.Entry,
) SecurityVerifier {
	return &securityContextConstraintsVerifier{
		validator:     validator,
		eventRecorder: eventRecorder,
		matchPolicies: matchPolicies,
		log:           log,
	}
}
