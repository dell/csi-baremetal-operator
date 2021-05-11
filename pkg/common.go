package pkg

import (
	"context"

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

var (
	crashVolume = corev1.Volume{
		Name: "crash-dump",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}}

	crashMountVolume = corev1.VolumeMount{
		Name: "crash-dump", MountPath: "/crash-dump",
	}
)

type CSIDeployment struct {
	node           Node
	controller     Controller
	extender       SchedulerExtender
	patcher        SchedulerPatcher
	nodeController NodeController
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
		nodeController: NodeController{
			Clientset: clientSet,
			Logger:    log.WithValues(CSIName, "nodeController"),
		},
	}
}

func (c *CSIDeployment) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	if err := c.node.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.controller.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.extender.Update(csi, scheme); err != nil {
		return err
	}

	if err := c.nodeController.Update(csi, scheme); err != nil {
		return err
	}

	// Patching method for the scheduler depends on the platform
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.PatchOpenShift(ctx, scheme)
	default:
		return c.patcher.Update(csi, scheme)

	}
}

func (c *CSIDeployment) UninstallPatcher(ctx context.Context, csi csibaremetalv1.Deployment) error {
	switch csi.Spec.Platform {
	case platformOpenshift:
		return c.patcher.UnPatchOpenShift(ctx)
	default:
		return nil
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

func makeNodeSelectorMap(ns *components.NodeSelector) map[string]string {
	if ns != nil {
		return map[string]string{ns.Key: ns.Value}
	}

	return map[string]string{}
}
