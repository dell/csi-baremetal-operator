package pkg

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	"github.com/go-logr/logr"
)

const (
	csiName = "csi-baremetal"
	componentName = "csi-baremetal-node"
	prometheusPort = "8787"
	serviceAccountName = "csi-node-sa"
	terminationGracePeriodSeconds = 10
	loopbackManagerConfigName = "loopback-config"
	
	// volumes
	csiSocketDirVolume = "csi-socket-dir"
	registrationDirVolume = "registration-dir"
)

type Node struct {
	kubernetes.Clientset
	log logr.Logger
}

func (n *Node) Create(namespace string) error {
	// todo when create resource we need to control it and revert any changes done by user manually
	dsClient := n.AppsV1().DaemonSets(namespace)
	// create daemonset
	ds := createNodeDaemonSet(namespace)
	if _, err := dsClient.Create(ds); err != nil {
		n.log.Error(err, "Failed to create daemon set")
		return err
	}

	n.log.Info("Daemon set created successfully")
	return nil
}

func createNodeDaemonSet(namespace string) *v1.DaemonSet {
	// todo split this definition
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: componentName, Namespace: namespace},
		Spec:       appsv1.DaemonSetSpec{
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": componentName},
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: map[string]string{
						"app": componentName,
						"app.kubernetes.io/name": csiName,
					},
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port": prometheusPort,
						"prometheus.io/path": "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Volumes:                       createNodeVolumes(),
					Containers:                    createNodeContainers(),
					// todo what is the hack?
					TerminationGracePeriodSeconds: pointer.Int64Ptr(terminationGracePeriodSeconds),
					NodeSelector:                  map[string]string{},
					ServiceAccountName:            serviceAccountName,
					HostIPC:                       true,
				},
			},
		},
	}
}

func createNodeVolumes() []corev1.Volume {
	// todo how to avoid this?
	directory := corev1.HostPathDirectory
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	
	return []corev1.Volume{
		{Name: "logs", VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
		{Name: "host-dev", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &directory},
		}},
		// todo this if for loopback manager only
		{Name: "host-home", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/home", Type: &directory},
		}},
		{Name: "host-sys", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/sys", Type: &directory},
		}},
		{Name: "host-root", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/", Type: &directory},
		}},
		{Name: "host-run-udev", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev", Type: &directory},
		}},
		{Name: "host-run-lvm", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/lvm", Type: &directory},
		}},
		{Name: "host-run-lock", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/lock", Type: &directory},
		}},
		{Name: csiSocketDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/csi-baremetal", Type: &directoryOrCreate},
		}},
		{Name: registrationDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry/", Type: &directoryOrCreate},
		}},
		{Name: "mountpoint-dir", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/pods", Type: &directory},
		}},
		{Name: "csi-path", VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/kubernetes.io/csi"},
		}},
		// todo optional
		{Name: "drive-config", VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{
				Name: loopbackManagerConfigName}},
		}},
	}
}

func createNodeContainers() []corev1.Container {
	return []corev1.Container{
		{
			Name: "liveness-probe",
		 	Image: "livenessprobe:v2.1.0",
		 	ImagePullPolicy: corev1.PullIfNotPresent,
		 	Args: []string{"--csi-address=/csi/csi.sock"},
		 	VolumeMounts: []corev1.VolumeMount{
				{Name: csiSocketDirVolume, MountPath: "/csi"},
			},
		},
		{
			Name: "csi-node-driver-registrar",
			Image: "csi-node-driver-registrar:v1.0.1-gke.0",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{"--v=5", "--csi-address=$(ADDRESS)", "--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)"},
			Lifecycle: &corev1.Lifecycle{PreStop: &corev1.Handler{Exec: &corev1.ExecAction{Command: []string{
				"/bin/sh", "-c", "rm -rf /registration/csi-baremetal /registration/csi-baremetal-reg.sock"}}}},
			Env: []corev1.EnvVar{
				{Name: "ADDRESS", Value: "/csi/csi.sock"},
				{Name: "DRIVER_REG_SOCK_PATH", Value: "/var/lib/kubelet/plugins/csi-baremetal/csi.sock"},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: csiSocketDirVolume, MountPath: "/csi"},
				{Name: registrationDirVolume, MountPath: "/registration"},
			},
		},
	}
}
