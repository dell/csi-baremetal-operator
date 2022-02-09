package securityverifier

import (
	"context"
	"errors"

	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/feature"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	"github.com/dell/csi-baremetal-operator/pkg/validator/models"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
	rbacmodels "github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

type podSecurityPolicyVerifier struct {
	validator           validator.Validator
	eventRecorder       events.EventRecorder
	matchPolicyTemplate rbacv1.PolicyRule
	log                 *logrus.Entry
}

func (v *podSecurityPolicyVerifier) Verify(ctx context.Context, csi *csibaremetalv1.Deployment, serviceAccount string) error {
	var policyRule = v.matchPolicyTemplate
	policyRule.ResourceNames = []string{csi.Spec.PodSecurityPolicy.ResourceName}
	return v.validator.ValidateRBAC(ctx, &models.RBACRules{
		Data: &rbacmodels.ServiceAccountIsRoleBoundData{
			ServiceAccountName: serviceAccount,
			Namespace:          csi.Namespace,
			Role: &rbacv1.Role{
				Rules: []rbacv1.PolicyRule{policyRule},
			},
		},
		Type: models.ServiceAccountIsRoleBound,
	})
}

func (v *podSecurityPolicyVerifier) HandleError(_ context.Context, csi *csibaremetalv1.Deployment, serviceAccount string, err error) error {
	var rbacError rbac.Error
	if errors.As(err, &rbacError) {
		v.eventRecorder.Eventf(csi, eventing.WarningType, "PodSecurityPolicyVerificationFailed",
			"ServiceAccount %s has insufficient pod security policies, should have privileged",
			serviceAccount)
		v.log.Warning(rbacError, "Service account has insufficient pod security policies, should have privileged")
		return NewVerifierError("Service account has insufficient pod security policies, should have privileged")
	}
	v.log.Error(err, "Error occurred while validating service account pod security policies bindings")
	return err
}

// NewPodSecurityPolicyVerifier is a constructor for pod security policies verifier
func NewPodSecurityPolicyVerifier(
	validator validator.Validator,
	eventRecorder events.EventRecorder,
	matchPolicyTemplate rbacv1.PolicyRule,
	log *logrus.Entry,
) feature.SecurityVerifier {
	return &podSecurityPolicyVerifier{
		validator:           validator,
		eventRecorder:       eventRecorder,
		matchPolicyTemplate: matchPolicyTemplate,
		log:                 log,
	}
}
