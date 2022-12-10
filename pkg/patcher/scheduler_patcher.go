package patcher

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	securityverifier "github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier"
)

// SchedulerPatcher performs pacthing procedure depends on platform
type SchedulerPatcher struct {
	Clientset                 kubernetes.Interface
	Log                       *logrus.Entry
	Client                    client.Client
	PodSecurityPolicyVerifier securityverifier.SecurityVerifier
	// OpenshiftMasterNodeIP used for openshift secondary scheduler extender config if applicable
	OpenshiftMasterNodeIP string
}

// Update updates or creates csi-baremetal-se-patcher on RKE and Vanilla
// patches Kube-Scheduler on Openshift
func (p *SchedulerPatcher) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	if !IsPatchingEnabled(csi) {
		p.Log.Warn("Kubernetes scheduler configuration patching not enabled. Please update configuration manually")
		return nil
	}

	var err error
	switch csi.Spec.Platform {
	case constant.PlatformOpenShift:
		err = p.patchOpenShift(ctx, csi)
		if err == nil && csi.Spec.SecondaryScheduler {
			err = p.patchOpenShiftSecondaryScheduler(ctx, csi)
		}
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
		err := p.unPatchOpenShift(ctx)
		if err == nil && csi.Spec.SecondaryScheduler {
			err = p.unPatchOpenShiftSecondaryScheduler(ctx)
		}
		return err
	}
	return nil
}
