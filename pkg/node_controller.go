package pkg

import (
	"context"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	nodeController     = "node-controller"
	nodeControllerName = constant.CSIName + "-" + nodeController
	ncReplicasCount    = 1

	nodeControllerServiceAccountName = "csi-node-controller-sa"
)

// NodeController controls csi-baremetal-node-controller
type NodeController struct {
	Clientset kubernetes.Interface
	logr.Logger
}

// Update updates csi-baremetal-node-controller or creates if not found
func (nc *NodeController) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// create deployment
	expected := createNodeControllerDeployment(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	if err := common.UpdateDeployment(ctx, nc.Clientset, expected, nc.Logger); err != nil {
		return err
	}

	return nil
}

func createNodeControllerDeployment(csi *csibaremetalv1.Deployment) *v1.Deployment {
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeControllerName,
			Namespace: csi.GetNamespace(),
			Labels:    common.ConstructLabelAppMap(),
		},
		Spec: v1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(ncReplicasCount),
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: common.ConstructSelectorMap(nodeControllerName),
			},
			// template
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: common.ConstructLabelMap(nodeControllerName),
				},
				Spec: corev1.PodSpec{
					Containers:                    createNodeControllerContainers(csi),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(constant.TerminationGracePeriodSeconds),
					ServiceAccountName:            nodeControllerServiceAccountName,
					DeprecatedServiceAccount:      nodeControllerServiceAccountName,
					SecurityContext:               &corev1.PodSecurityContext{},
					ImagePullSecrets:              common.MakeImagePullSecrets(csi.Spec.RegistrySecret),
					SchedulerName:                 corev1.DefaultSchedulerName,
					Volumes:                       []corev1.Volume{constant.CrashVolume},
				},
			},
		},
	}
}

func createNodeControllerContainers(csi *csibaremetalv1.Deployment) []corev1.Container {
	var (
		image = csi.Spec.NodeController.Image
		log   = csi.Spec.NodeController.Log
		ns    = csi.Spec.NodeSelector
	)

	args := []string{
		"--namespace=$(NAMESPACE)",
		"--loglevel=" + common.MatchLogLevel(log.Level),
		"--logformat=" + common.MatchLogFormat(log.Format),
	}
	if ns != nil {
		args = append(args, "--nodeselector="+ns.Key+":"+ns.Value)
	}

	return []corev1.Container{
		{
			Name:            nodeController,
			Image:           common.ConstructFullImageName(image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Args:            args,
			Env: []corev1.EnvVar{
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
			},
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
			VolumeMounts:             []corev1.VolumeMount{constant.CrashMountVolume},
		},
	}
}
