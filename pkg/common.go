package pkg

import (
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal/pkg/base/featureconfig"
	"github.com/go-logr/logr"
)

const (
	CSIName = "csi-baremetal"
	// versions
	CSIVersion = "0.0.13-375.3c20841"

	// ports
	PrometheusPort = 8787
	LivenessPort   = "liveness-port"

	// timeouts
	TerminationGracePeriodSeconds = 10

	// volumes
	LogsVolume         = "logs"
	CSISocketDirVolume = "csi-socket-dir"

	// feature flags
	UseNodeAnnotation = false
)

type CSIDeployment struct {
	node       Node
	controller Controller
	extender   SchedulerExtender
	patcher    SchedulerPatcher
}

func NewCSIDeployment(clientSet kubernetes.Clientset, log logr.Logger, fchecker featureconfig.FeatureChecker) CSIDeployment {
	return CSIDeployment{
		node: Node{
			Clientset:      clientSet,
			Logger:         log.WithValues(CSIName, "node"),
			FeatureChecker: fchecker,
		},
		controller: Controller{
			Clientset: clientSet,
			Logger:    log.WithValues(CSIName, "controller"),
		},
		extender: SchedulerExtender{
			Clientset: clientSet,
			Logger:    log.WithValues(CSIName, "extender"),
		},
		patcher: SchedulerPatcher{
			Clientset: clientSet,
			Logger:    log.WithValues(CSIName, "patcher"),
		},
	}
}

func (c *CSIDeployment) Update(csi *csibaremetalv1.Deployment) error {
	if err := c.node.Update(csi); err != nil {
		return err
	}

	if err := c.controller.Update(csi); err != nil {
		return err
	}

	if err := c.extender.Update(csi); err != nil {
		return err
	}

	if err := c.patcher.Update(csi); err != nil {
		return err
	}

	return nil
}

func GetNamespace(csi *csibaremetalv1.Deployment) string {
	if csi.Namespace == "" {
		return "default"
	}

	return csi.Namespace
}

func isDaemonSetDeployed(dsClient appsv1.DaemonSetInterface, name string) (bool, error) {
	_, err := dsClient.Get(name, metav1.GetOptions{})
	return isFound(err)
}

func isDeploymentDeployed(dsClient appsv1.DeploymentInterface, name string) (bool, error) {
	_, err := dsClient.Get(name, metav1.GetOptions{})
	return isFound(err)
}

func isFound(err error) (bool, error) {
	if err != nil {
		if k8sError.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
