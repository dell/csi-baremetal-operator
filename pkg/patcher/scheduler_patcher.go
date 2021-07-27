package patcher

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

// SchedulerPatcher performs pacthing procedure depends on platform
type SchedulerPatcher struct {
	Clientset kubernetes.Interface
	logr.Logger
	Client client.Client
}

// Update updates or creates csi-baremetal-se-patcher on RKE and Vanilla
// patches Kube-Scheduler on Openshift
func (p *SchedulerPatcher) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	var err error

	switch csi.Spec.Platform {
	case PlatformOpenshift:
		err = p.patchOpenShift(ctx, csi, scheme)
	case PlatformVanilla, PlatformRKE:
		err = p.updateVanilla(ctx, csi, scheme)
	default:
		p.Logger.Info("Platform is unavailable or not set. Patching disabled")
		return nil
	}
	if err != nil {
		return err
	}

	return p.UpdateReadinessConfigMap(ctx, csi, scheme)
}

// Uninstall unpatch Openshift Scheduler
func (p *SchedulerPatcher) Uninstall(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	switch csi.Spec.Platform {
	case PlatformOpenshift:
		return p.unPatchOpenShift(ctx)
	default:
		return nil
	}
}
