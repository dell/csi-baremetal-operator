package pkg

import (
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	"github.com/go-logr/logr"
)

const (
	nodeName                  = csiName + "-node"
	nodeServiceAccountName    = "csi-node-sa"
	loopbackManagerConfigName = "loopback-config"

	// feature flags
	useNodeAnnotation = false

	// ports
	driveManagerPort = 8888

	// volumes
	registrationDirVolume = "registration-dir"
	hostDevVolume         = "host-dev"
	hostHomeVolume        = "host-home"
	hostSysVolume         = "host-sys"
	hostRootVolume        = "host-root"
	hostRunUdevVolume     = "host-run-udev"
	hostRunLVMVolume      = "host-run-lvm"
	hostRunLock           = "host-run-lock"
	mountPointDirVolume   = "mountpoint-dir"
	csiPathVolume         = "csi-path"
	driveConfigVolume     = "drive-config"
)

type Node struct {
	kubernetes.Clientset
	logr.Logger
}

// todo add rbac
func (n *Node) Create(namespace string) error {
	// todo when create resource we need to control it and revert any changes done by user manually
	dsClient := n.AppsV1().DaemonSets(namespace)

	// create daemonset
	ds := createNodeDaemonSet(namespace)
	if _, err := dsClient.Create(ds); err != nil {
		n.Logger.Error(err, "Failed to create daemon set")
		return err
	}

	n.Logger.Info("Daemon set created successfully")
	return nil
}

func createNodeDaemonSet(namespace string) *v1.DaemonSet {
	// todo split this definition
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, Namespace: namespace},
		Spec: appsv1.DaemonSetSpec{
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": nodeName},
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: map[string]string{
						"app":                    nodeName,
						"app.kubernetes.io/name": csiName,
					},
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(prometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Volumes:    createNodeVolumes(),
					Containers: createNodeContainers(),
					// todo what is the hack?
					TerminationGracePeriodSeconds: pointer.Int64Ptr(terminationGracePeriodSeconds),
					// todo fill in selectors when passed
					NodeSelector:       map[string]string{},
					ServiceAccountName: nodeServiceAccountName,
					HostIPC:            true,
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
		{Name: logsVolume, VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
		{Name: hostDevVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &directory},
		}},
		// todo this if for loopback manager only
		{Name: hostHomeVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/home", Type: &directory},
		}},
		{Name: hostSysVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/sys", Type: &directory},
		}},
		{Name: hostRootVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/", Type: &directory},
		}},
		{Name: hostRunUdevVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev", Type: &directory},
		}},
		{Name: hostRunLVMVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/lvm", Type: &directory},
		}},
		{Name: hostRunLock, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/lock", Type: &directory},
		}},
		{Name: csiSocketDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/csi-baremetal", Type: &directoryOrCreate},
		}},
		{Name: registrationDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry/", Type: &directoryOrCreate},
		}},
		{Name: mountPointDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/pods", Type: &directory},
		}},
		{Name: csiPathVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/kubernetes.io/csi"},
		}},
		// todo optional
		{Name: driveConfigVolume, VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: loopbackManagerConfigName},
				Optional:             pointer.BoolPtr(true),
			},
		}},
	}
}

func createNodeContainers() []corev1.Container {
	// todo get rid of this hack
	bidirectional := corev1.MountPropagationBidirectional
	return []corev1.Container{
		{
			Name:            "liveness-probe",
			Image:           "livenessprobe:v2.1.0",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args:            []string{"--csi-address=/csi/csi.sock"},
			VolumeMounts: []corev1.VolumeMount{
				{Name: csiSocketDirVolume, MountPath: "/csi"},
			},
		},
		{
			Name:            "csi-node-driver-registrar",
			Image:           "csi-node-driver-registrar:v1.0.1-gke.0",
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{"--v=5", "--csi-address=$(ADDRESS)",
				"--kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)"},
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
		{
			Name:            "node",
			Image:           "csi-baremetal-node:" + csiVersion,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"--csiendpoint=$(CSI_ENDPOINT)",
				"--nodename=$(KUBE_NODE_NAME)",
				"--namespace=$(NAMESPACE)",
				"--extender=true",
				"--usenodeannotation=" + strconv.FormatBool(useNodeAnnotation),
				"--loglevel=info",
				"--metrics-address=:" + strconv.Itoa(prometheusPort),
				"--metrics-path=/metrics",
				"--drivemgrendpoint=tcp://localhost:" + strconv.Itoa(driveManagerPort),
			},
			Ports: []corev1.ContainerPort{
				{Name: livenessPort, ContainerPort: 9808, Protocol: corev1.ProtocolTCP},
				{Name: "metrics", ContainerPort: prometheusPort, Protocol: corev1.ProtocolTCP},
			},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString(livenessPort)}},
				InitialDelaySeconds: 300,
				TimeoutSeconds:      3,
				PeriodSeconds:       10,
				FailureThreshold:    5,
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{Exec: &corev1.ExecAction{Command: []string{
					"/health_probe",
					"-addr=:9999"}}},
				InitialDelaySeconds: 3,
				PeriodSeconds:       3,
				SuccessThreshold:    3,
				FailureThreshold:    100,
			},
			Env: []corev1.EnvVar{
				{Name: "CSI_ENDPOINT", Value: "unix:///csi/csi.sock"},
				{Name: "LOG_FORMAT", Value: "text"},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
				{Name: "MY_POD_IP", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
				}},
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
			},
			SecurityContext: &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			VolumeMounts: []corev1.VolumeMount{
				{Name: logsVolume, MountPath: "/var/log"},
				{Name: hostDevVolume, MountPath: "/dev"},
				{Name: hostSysVolume, MountPath: "/sys"},
				{Name: hostRunUdevVolume, MountPath: "/run/udev"},
				{Name: hostRunLVMVolume, MountPath: "/run/lvm"},
				{Name: hostRunLock, MountPath: "/run/lock"},
				{Name: csiSocketDirVolume, MountPath: "/csi"},
				{Name: mountPointDirVolume, MountPath: "/var/lib/kubelet/pods", MountPropagation: &bidirectional},
				{Name: csiPathVolume, MountPath: "/var/lib/kubelet/plugins/kubernetes.io/csi", MountPropagation: &bidirectional},
				{Name: hostRootVolume, MountPath: "/hostroot", MountPropagation: &bidirectional},
			},
		},
		{
			Name:            "drivemgr",
			Image:           "csi-baremetal-loopbackmgr:" + csiVersion,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Args: []string{
				"--loglevel=info",
				"--drivemgrendpoint=tcp://localhost:" + strconv.Itoa(driveManagerPort),
				"--usenodeannotation=" + strconv.FormatBool(useNodeAnnotation),
			},
			Env: []corev1.EnvVar{
				{Name: "LOG_FORMAT", Value: "text"},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
			},
			SecurityContext: &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			VolumeMounts: []corev1.VolumeMount{
				{Name: hostDevVolume, MountPath: "/dev"},
				{Name: hostHomeVolume, MountPath: "/host/home"},
				{Name: driveConfigVolume, MountPath: "/etc/config"},
			},
		},
	}
}
