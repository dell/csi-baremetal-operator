package common

import (
	"context"

	openshiftv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/constant"

	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	accrd "github.com/dell/csi-baremetal/api/v1/availablecapacitycrd"
	"github.com/dell/csi-baremetal/api/v1/drivecrd"
	"github.com/dell/csi-baremetal/api/v1/lvgcrd"
	"github.com/dell/csi-baremetal/api/v1/nodecrd"
	"github.com/dell/csi-baremetal/api/v1/volumecrd"
)

// MatchLogLevel checks if passed logLevel is allowed
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

// MatchLogFormat checks if passed logFormat is allowed
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

// ConstructFullImageName returns name of image in the following format: <registry>/<image_name>:<image_tag>
func ConstructFullImageName(image *components.Image, registry string) string {
	var imageName string

	if registry != "" {
		imageName += registry + "/"
	}

	imageName += image.Name + ":" + image.Tag
	return imageName
}

// MakeNodeSelectorMap creates map with node selector from csi spec
func MakeNodeSelectorMap(ns *components.NodeSelector) map[string]string {
	if ns != nil {
		return map[string]string{ns.Key: ns.Value}
	}

	return map[string]string{}
}

// MakeImagePullSecrets creates list with imagePullSecret from csi spec
func MakeImagePullSecrets(rs string) []corev1.LocalObjectReference {
	if len(rs) != 0 {
		return []corev1.LocalObjectReference{{Name: rs}}
	}

	return []corev1.LocalObjectReference{}
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

// PrepareScheme returns a scheme to manager setup
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

// ConstructLabelMap creates the map contains pod labels
func ConstructLabelMap(appName string) map[string]string {
	labels := ConstructLabelAppMap()
	labels[constant.SelectorKey] = appName
	labels[constant.FluentbitLabelKey] = appName
	return labels
}

// ConstructLabelAppMap creates the map contains app labels
func ConstructLabelAppMap() map[string]string {
	return map[string]string{
		constant.AppLabelKey:      constant.AppLabelValue,
		constant.AppLabelShortKey: constant.AppLabelValue,
	}
}

// ConstructSelectorMap creates the map contains deployment/daemonset selectors
func ConstructSelectorMap(appName string) map[string]string {
	return map[string]string{
		constant.SelectorKey: appName,
	}
}

// ConstructResourceRequirements creates the ResourceRequirements contains Limits and Requests lists
func ConstructResourceRequirements(resources *components.ResourceRequirements) corev1.ResourceRequirements {
	if resources != nil {
		return corev1.ResourceRequirements(*resources)
	}
	return corev1.ResourceRequirements{}
}
