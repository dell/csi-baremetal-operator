package pkg

import (
	"strconv"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
		found.Spec = expected.Spec
		if _, err := dsClient.Update(found); err != nil {
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
						// release label used by fluentbit to make "release" folder
						"release": extenderName,
					},
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Containers:                    createExtenderContainers(csi),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(TerminationGracePeriodSeconds),
					ServiceAccountName:            extenderServiceAccountName,
					DeprecatedServiceAccount:      extenderServiceAccountName,
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 corev1.DefaultSchedulerName,
					HostNetwork:                   true,
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
					Volumes: []corev1.Volume{crashVolume},
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
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
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
				{Name: "metrics", HostPort: PrometheusPort, ContainerPort: PrometheusPort, Protocol: corev1.ProtocolTCP},
				{Name: "extender", HostPort: extenderPort, ContainerPort: extenderPort, Protocol: corev1.ProtocolTCP},
			},
			TerminationMessagePath:   defaultTerminationMessagePath,
			TerminationMessagePolicy: defaultTerminationMessagePolicy,
			VolumeMounts:             []corev1.VolumeMount{crashMountVolume},
		},
	}
}
