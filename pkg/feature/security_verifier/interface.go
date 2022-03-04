package securityverifier

import (
	"context"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier/models"
)

// SecurityVerifier is an interface, describing security verifiers
type SecurityVerifier interface {
	Verify(ctx context.Context, csi *csibaremetalv1.Deployment, component models.Component) error
	HandleError(ctx context.Context, csi *csibaremetalv1.Deployment, serviceAccount string, err error) error
}
