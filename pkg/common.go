package pkg

import (
	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
)

const (
	CSIName = "csi-baremetal"

	// ports
	PrometheusPort = 8787
	LivenessPort   = "liveness-port"

	// timeouts
	TerminationGracePeriodSeconds = 10

	// volumes
	LogsVolume         = "logs"
	CSISocketDirVolume = "csi-socket-dir"

	// termination settings
	defaultTerminationMessagePath   = "/var/log/termination-log"
	defaultTerminationMessagePolicy = corev1.TerminationMessageReadFile
)

type CSIDeployment struct {
	node       Node
	controller Controller
	extender   SchedulerExtender
	patcher    SchedulerPatcher
}

func NewCSIDeployment(clientSet kubernetes.Clientset, client client.Client, log logr.Logger) CSIDeployment {
	return CSIDeployment{
		node: Node{
			Clientset: clientSet,
			Logger:    log.WithValues(CSIName, "node"),
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
			Client:    client,
			Logger:    log.WithValues(CSIName, "patcher"),
		},
	}
}

func (c *CSIDeployment) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	if err := c.node.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.controller.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.extender.Update(csi, scheme); err != nil {
		return err
	}

	// Patching method for the scheduler depends on the platform
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.UpdateOpenShift(csi, scheme)
	default:
		return c.patcher.Update(csi, scheme)

	}
}

func GetNamespace(csi *csibaremetalv1.Deployment) string {
	if csi.Namespace == "" {
		return "default"
	}

	return csi.Namespace
}

func deploymentChanged(expected *v1.Deployment, found *v1.Deployment) bool {
	if !equality.Semantic.DeepEqual(expected.Spec.Replicas, found.Spec.Replicas) {
		return true
	}

	if !equality.Semantic.DeepEqual(expected.Spec.Selector, found.Spec.Selector) {
		return true
	}

	if !equality.Semantic.DeepEqual(expected.Spec.Template, found.Spec.Template) {
		return true
	}

	return false
}

func daemonsetChanged(expected *v1.DaemonSet, found *v1.DaemonSet) bool {
	if !equality.Semantic.DeepEqual(expected.Spec.Selector, found.Spec.Selector) {
		return true
	}

	if !equality.Semantic.DeepEqual(expected.Spec.Template, found.Spec.Template) {
		return true
	}

	return false
}

func matchLogLevel(level components.Level) string {
	switch level {
	case components.InfoLevel:
		return string(level)
	case components.DebugLevel:
		return string(level)
	case components.TraceLevel:
		return string(level)

	default:
		return string(components.InfoLevel)
	}
}

func matchLogFormat(format components.Format) string {
	switch format {
	case components.JSONFormat:
		return string(format)
	case components.TextFormat:
		return string(format)

	default:
		return string(components.TextFormat)
	}
}

func constructFullImageName(image *components.Image, registry string) string {
	var imageName string

	if registry != "" {
		imageName += registry + "/"
	}

	imageName += image.Name + ":" + image.Tag
	return imageName
}
