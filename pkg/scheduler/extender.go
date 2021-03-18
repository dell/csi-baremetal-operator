package scheduler

import (
	"strconv"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/dell/csi-baremetal-operator/pkg"
	"github.com/go-logr/logr"
)

const (
	extenderContainerName      = "scheduler-extender"
	extenderImageName          = pkg.CSIName + "-" + extenderContainerName
	extenderName               = pkg.CSIName + "-se"
	extenderServiceAccountName = pkg.CSIName + "-extender-sa"

	extenderPort = 8889
)

type Extender struct {
	kubernetes.Clientset
	logr.Logger
}

// todo add rbac
func (n *Extender) Create(namespace string) error {
	dsClient := n.AppsV1().DaemonSets(namespace)

	// create daemonset
	ds := createExtenderDaemonSet(namespace)
	if _, err := dsClient.Create(ds); err != nil {
		n.Logger.Error(err, "Failed to create daemon set")
		return err
	}

	n.Logger.Info("Daemon set created successfully")
	return nil
}

func createExtenderDaemonSet(namespace string) *v1.DaemonSet {
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: extenderName, Namespace: namespace},
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
						"app.kubernetes.io/name": pkg.CSIName,
					},
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(pkg.PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Containers:         createExtenderContainers(),
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

func createExtenderContainers() []corev1.Container {
	return []corev1.Container{
		{
			Name:            extenderContainerName,
			Image:           extenderImageName + ":" + pkg.CSIVersion,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"--namespace=$(NAMESPACE)",
				"--provisioner=" + pkg.CSIName,
				"--port=" + strconv.Itoa(extenderPort),
				"--loglevel=debug",
				"--certFile=",
				"--privateKeyFile=",
				"--metrics-address=:" + strconv.Itoa(pkg.PrometheusPort),
				"--metrics-path=/metrics",
				"--usenodeannotation=" + strconv.FormatBool(pkg.UseNodeAnnotation),
			},
			Env: []corev1.EnvVar{
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
				{Name: "LOG_FORMAT", Value: "text"},
			},
			Ports: []corev1.ContainerPort{
				{ContainerPort: extenderPort},
				{Name: "metrics", ContainerPort: pkg.PrometheusPort, Protocol: corev1.ProtocolTCP},
			},
		},
	}
}
