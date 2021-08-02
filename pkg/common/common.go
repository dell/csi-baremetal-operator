package common

import (
	"context"

	openshiftv1 "github.com/openshift/api/config/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"

	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

func GetNamespace(csi *csibaremetalv1.Deployment) string {
	if csi.Namespace == "" {
		return "default"
	}

	return csi.Namespace
}

func DeploymentChanged(expected *v1.Deployment, found *v1.Deployment) bool {
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

func DaemonsetChanged(expected *v1.DaemonSet, found *v1.DaemonSet) bool {
	if !equality.Semantic.DeepEqual(expected.Spec.Selector, found.Spec.Selector) {
		return true
	}

	if !equality.Semantic.DeepEqual(expected.Spec.Template, found.Spec.Template) {
		return true
	}

	return false
}

func MatchLogLevel(level components.Level) string {
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

func MatchLogFormat(format components.Format) string {
	switch format {
	case components.JSONFormat:
		return string(format)
	case components.TextFormat:
		return string(format)

	default:
		return string(components.TextFormat)
	}
}

func ConstructFullImageName(image *components.Image, registry string) string {
	var imageName string

	if registry != "" {
		imageName += registry + "/"
	}

	imageName += image.Name + ":" + image.Tag
	return imageName
}

func MakeNodeSelectorMap(ns *components.NodeSelector) map[string]string {
	if ns != nil {
		return map[string]string{ns.Key: ns.Value}
	}

	return map[string]string{}
}

// GetSelectedNodes returns a list of nodes filtered with NodeSelector
func GetSelectedNodes(ctx context.Context, c kubernetes.Interface, selector *components.NodeSelector) (*corev1.NodeList, error) {
	var listOptions = metav1.ListOptions{}

	if selector != nil {
		labelSelector := metav1.LabelSelector{MatchLabels: MakeNodeSelectorMap(selector)}
		listOptions.LabelSelector = labels.Set(labelSelector.MatchLabels).String()
	}

	nodes, err := c.CoreV1().Nodes().List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func PrepareScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := openshiftv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err := csibaremetalv1.AddToScheme(scheme); err != nil {
		return nil, err
	}

	// CSI resources
	if err := nodecrd.AddToSchemeCSIBMNode(scheme); err != nil {
		return nil, err
	}
	if err := volumecrd.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := drivecrd.AddToSchemeDrive(scheme); err != nil {
		return nil, err
	}
	if err := lvgcrd.AddToSchemeLVG(scheme); err != nil {
		return nil, err
	}
	if err := accrd.AddToSchemeAvailableCapacity(scheme); err != nil {
		return nil, err
	}
	if err := acrcrd.AddToSchemeACR(scheme); err != nil {
		return nil, err
	}

	return scheme, nil
}
