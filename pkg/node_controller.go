package pkg

import (
	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

const (
	nodeController     = "node-controller"
	nodeControllerName = CSIName + "-" + nodeController
	ncReplicasCount    = 1

	nodeControllerServiceAccountName = "csi-node-controller-sa"
)

type NodeController struct {
	kubernetes.Clientset
	logr.Logger
}

func (nc *NodeController) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// create deployment
	expected := createNodeControllerDeployment(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	namespace := GetNamespace(csi)
	dsClient := nc.AppsV1().Deployments(namespace)

	found, err := dsClient.Get(nodeControllerName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if _, err := dsClient.Create(expected); err != nil {
				nc.Logger.Error(err, "Failed to create deployment")
				return err
			}

			nc.Logger.Info("Deployment created successfully")
			return nil
		}

		nc.Logger.Error(err, "Failed to get deployment")
		return err
	}

	if deploymentChanged(expected, found) {
		found.Spec = expected.Spec
		if _, err := dsClient.Update(found); err != nil {
			nc.Logger.Error(err, "Failed to update deployment")
			return err
		}

		nc.Logger.Info("Deployment updated successfully")
		return nil
	}

	return nil
}

func createNodeControllerDeployment(csi *csibaremetalv1.Deployment) *v1.Deployment {
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeControllerName,
			Namespace: GetNamespace(csi),
		},
		Spec: v1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(ncReplicasCount),
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": nodeControllerName},
			},
			// template
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: map[string]string{
						"app": nodeControllerName,
						// release label used by fluentbit to make "release" folder
						"release": nodeControllerName,
					},
				},
				Spec: corev1.PodSpec{
					Containers:                    createNodeControllerContainers(csi),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(TerminationGracePeriodSeconds),
					ServiceAccountName:            nodeControllerServiceAccountName,
					DeprecatedServiceAccount:      nodeControllerServiceAccountName,
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 corev1.DefaultSchedulerName,
					Volumes:                       []corev1.Volume{crashVolume},
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
		"--loglevel=" + matchLogLevel(log.Level),
		"--logformat=" + matchLogFormat(log.Format),
	}
	if ns != nil {
		args = append(args, "--nodeselector="+ns.Key+":"+ns.Value)
	}

	return []corev1.Container{
		{
			Name:            nodeController,
			Image:           constructFullImageName(image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Args:            args,
			Env: []corev1.EnvVar{
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
			},
			TerminationMessagePath:   defaultTerminationMessagePath,
			TerminationMessagePolicy: defaultTerminationMessagePolicy,
			VolumeMounts:             []corev1.VolumeMount{crashMountVolume},
		},
	}
}
