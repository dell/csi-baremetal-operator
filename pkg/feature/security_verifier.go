package feature

import (
	"context"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

type SecurityVerifier interface {
	Verify(ctx context.Context, csi *csibaremetalv1.Deployment, serviceAccount string) error
	HandleError(ctx context.Context, csi *csibaremetalv1.Deployment, serviceAccount string, err error) error
}
