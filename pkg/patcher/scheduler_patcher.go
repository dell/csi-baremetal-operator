package patcher

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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
	// will only set on Openshift
	KubernetesVersion string
	// whether use secondary scheduler on Openshift
	UseOpenshiftSecondaryScheduler bool
	// SelectedSchedulerExtenderIP used for openshift secondary scheduler extender config if applicable
	SelectedSchedulerExtenderIP string
	// HTTPClient used for openshift secondary scheduler extender config if applicable
	HTTPClient *http.Client
}

func (p *SchedulerPatcher) useOpenshiftSecondaryScheduler(platform string) (bool, error) {
	if platform == constant.PlatformOpenShift && p.KubernetesVersion == "" {
		if k8sVersionInfo, err := p.Clientset.Discovery().ServerVersion(); err == nil {
			var (
				k8sMajorVersion int
				k8sMinorVersion int
			)
			k8sMajorVersion, err = strconv.Atoi(k8sVersionInfo.Major)
			if err != nil {
				return false, err
			}
			k8sMinorVersion, err = strconv.Atoi(k8sVersionInfo.Minor)
			if err != nil {
				return false, err
			}
			p.KubernetesVersion = fmt.Sprintf("%d.%d", k8sMajorVersion, k8sMinorVersion)
			p.Log.Infof("Kubernetes version: %s", p.KubernetesVersion)
			p.UseOpenshiftSecondaryScheduler = k8sMajorVersion == 1 && k8sMinorVersion > 22
		} else {
			return false, err
		}
	}
	return p.UseOpenshiftSecondaryScheduler, nil
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
