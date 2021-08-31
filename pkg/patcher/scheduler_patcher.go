package patcher

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
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
	if !IsPatchingEnabled(csi) {
		// todo change severity to warning once https://github.com/dell/csi-baremetal/issues/371 is addressed
		p.Logger.Info("Kubernetes scheduler configuration patching not enabled. Please update configuration manually")
		return nil
	}

	var err error
	switch csi.Spec.Platform {
	case constant.PlatformOpenShift:
		err = p.patchOpenShift(ctx, csi)
	case constant.PlatformVanilla, constant.PlatformRKE:
		err = p.updateVanilla(ctx, csi, scheme)
	}
	if err != nil {
		return err
	}

	return p.UpdateReadinessConfigMap(ctx, csi, scheme)
}

// Uninstall unpatch Openshift Scheduler
func (p *SchedulerPatcher) Uninstall(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	if IsPatchingEnabled(csi) && csi.Spec.Platform == constant.PlatformOpenShift {
		return p.unPatchOpenShift(ctx)
	}
	return nil
}
