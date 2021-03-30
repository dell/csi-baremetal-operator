package pkg

import (
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/go-logr/logr"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

const (
	extenderContainerName      = "scheduler-extender"
	extenderName               = CSIName + "-se"
	extenderServiceAccountName = CSIName + "-extender-sa"

	extenderPort = 8889
)

type SchedulerExtender struct {
	kubernetes.Clientset
	logr.Logger
}

func (n *SchedulerExtender) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// create daemonset
	expected := createExtenderDaemonSet(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	namespace := GetNamespace(csi)
	dsClient := n.AppsV1().DaemonSets(namespace)

	found, err := dsClient.Get(extenderName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if _, err := dsClient.Create(expected); err != nil {
				n.Logger.Error(err, "Failed to create daemonset")
				return err
			}

			n.Logger.Info("Daemonset created successfully")
			return nil
		}

		n.Logger.Error(err, "Failed to get daemonset")
		return err
	}

	if daemonsetChanged(expected, found) {
		if _, err := dsClient.Update(expected); err != nil {
			n.Logger.Error(err, "Failed to update daemonset")
			return err
		}

		n.Logger.Info("Daemonset updated successfully")
		return nil
	}

	return nil
}

func createExtenderDaemonSet(csi *csibaremetalv1.Deployment) *v1.DaemonSet {
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      extenderName,
			Namespace: GetNamespace(csi),
		},
		Spec: v1.DaemonSetSpec{
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": extenderName},
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: map[string]string{
						"app":                    extenderName,
						"app.kubernetes.io/name": CSIName,
					},
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Containers:         createExtenderContainers(csi),
					ServiceAccountName: extenderServiceAccountName,
					HostNetwork:        true,
					Tolerations: []corev1.Toleration{
						{Key: "CriticalAddonsOnly", Operator: corev1.TolerationOpExists},
						{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule},
					},
					Affinity: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
							NodeSelectorTerms: []corev1.NodeSelectorTerm{
								{MatchExpressions: []corev1.NodeSelectorRequirement{
									{Key: "node-role.kubernetes.io/master", Operator: corev1.NodeSelectorOpExists},
								}},
							}},
					}},
				},
			},
		},
	}
}

func createExtenderContainers(csi *csibaremetalv1.Deployment) []corev1.Container {
	return []corev1.Container{
		{
			Name:            extenderContainerName,
			Image:           constructFullImageName(csi.Spec.Scheduler.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.Scheduler.Image.PullPolicy),
			Args: []string{
				"--namespace=$(NAMESPACE)",
				"--provisioner=" + CSIName,
				"--port=" + strconv.Itoa(extenderPort),
				"--loglevel=" + matchLogLevel(csi.Spec.Scheduler.Log.Level),
				"--certFile=",
				"--privateKeyFile=",
				"--metrics-address=:" + strconv.Itoa(PrometheusPort),
				"--metrics-path=/metrics",
				"--usenodeannotation=" + strconv.FormatBool(csi.Spec.NodeIDAnnotation),
			},
			Env: []corev1.EnvVar{
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
				{Name: "LOG_FORMAT", Value: matchLogFormat(csi.Spec.Scheduler.Log.Format)},
			},
			Ports: []corev1.ContainerPort{
				{ContainerPort: extenderPort},
				{Name: "metrics", ContainerPort: PrometheusPort, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}
