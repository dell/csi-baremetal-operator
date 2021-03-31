package pkg

import (
	"strconv"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

const (
	nodeName                  = CSIName + "-node"
	nodeServiceAccountName    = "csi-node-sa"
	loopbackManagerConfigName = "loopback-config"

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

	livenessProbeSidecar   = "livenessprobe"
	driverRegistrarSidecar = "csi-node-driver-registrar"
)

type Node struct {
	kubernetes.Clientset
	logr.Logger
}

func (n *Node) Update(csi *csibaremetalv1.Deployment) error {
	namespace := GetNamespace(csi)
	dsClient := n.AppsV1().DaemonSets(namespace)

	isDeployed, err := isDaemonSetDeployed(dsClient, nodeName)
	if err != nil {
		n.Logger.Error(err, "Failed to get daemon set")
		return err
	}

	if isDeployed {
		n.Logger.Info("Daemon set already deployed")
		return nil
	}

	// create daemonset
	ds := createNodeDaemonSet(csi)
	if _, err := dsClient.Create(ds); err != nil {
		n.Logger.Error(err, "Failed to create daemon set")
		return err
	}

	n.Logger.Info("Daemon set created successfully")
	return nil
}

func createNodeDaemonSet(csi *csibaremetalv1.Deployment) *v1.DaemonSet {
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: GetNamespace(csi),
		},
		Spec: v1.DaemonSetSpec{
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
					Volumes:                       createNodeVolumes(csi.Spec.GlobalRegistry == ""),
					Containers:                    createNodeContainers(csi),
					TerminationGracePeriodSeconds: pointer.Int64Ptr(TerminationGracePeriodSeconds),
					NodeSelector:                  csi.Spec.NodeSelectors,
					ServiceAccountName:            nodeServiceAccountName,
					HostIPC:                       true,
				},
			},
		},
	}
}

func createNodeVolumes(deployConfig bool) []corev1.Volume {
	directory := corev1.HostPathDirectory
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	volumes := []corev1.Volume{
		{Name: LogsVolume, VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
		{Name: hostDevVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &directory},
		}},
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
		{Name: CSISocketDirVolume, VolumeSource: corev1.VolumeSource{
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
	}

	if deployConfig {
		volumes = append(volumes, corev1.Volume{
			Name: driveConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: loopbackManagerConfigName},
					Optional:             pointer.BoolPtr(true),
				},
			}})
	}
	return volumes
}

// todo split long methods - https://github.com/dell/csi-baremetal/issues/329
func createNodeContainers(csi *csibaremetalv1.Deployment) []corev1.Container {
	var (
		bidirectional = corev1.MountPropagationBidirectional
		driveMgr      = csi.Spec.Driver.Node.DriveMgr
		node          = csi.Spec.Driver.Node
		testEnv       = csi.Spec.GlobalRegistry == ""
		lp            = node.Sidecars[livenessProbeSidecar]
		dr            = node.Sidecars[driverRegistrarSidecar]
	)
	args := []string{
		"--loglevel=" + matchLogLevel(node.Log.Level),
		"--drivemgrendpoint=" + driveMgr.Endpoint,
	}
	mounts := []corev1.VolumeMount{
		{Name: hostDevVolume, MountPath: "/dev"},
		{Name: hostHomeVolume, MountPath: "/host/home"},
	}
	if testEnv {
		mounts = append(mounts, corev1.VolumeMount{Name: driveConfigVolume, MountPath: "/etc/config"})
		args = append(args, "--usenodeannotation="+strconv.FormatBool(csi.Spec.NodeIDAnnotation))
	}
	return []corev1.Container{
		{
			Name:            livenessProbeSidecar,
			Image:           constructFullImageName(lp.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(lp.Image.PullPolicy),
			Args:            []string{"--csi-address=/csi/csi.sock"},
			VolumeMounts: []corev1.VolumeMount{
				{Name: CSISocketDirVolume, MountPath: "/csi"},
			},
		},
		{
			Name:            driverRegistrarSidecar,
			Image:           constructFullImageName(dr.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(dr.Image.PullPolicy),
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
				{Name: CSISocketDirVolume, MountPath: "/csi"},
				{Name: registrationDirVolume, MountPath: "/registration"},
			},
		},
		{
			Name:            "node",
			Image:           constructFullImageName(node.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(node.Image.PullPolicy),
			Args: []string{
				"--csiendpoint=$(CSI_ENDPOINT)",
				"--nodename=$(KUBE_NODE_NAME)",
				"--namespace=$(NAMESPACE)",
				"--extender=true",
				"--loglevel=" + matchLogLevel(node.Log.Level),
				"--metrics-address=:" + strconv.Itoa(PrometheusPort),
				"--metrics-path=/metrics",
				"--drivemgrendpoint=" + driveMgr.Endpoint,
			},
			Ports: []corev1.ContainerPort{
				{Name: LivenessPort, ContainerPort: 9808, Protocol: corev1.ProtocolTCP},
				{Name: "metrics", ContainerPort: PrometheusPort, Protocol: corev1.ProtocolTCP},
			},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString(LivenessPort)}},
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
				{Name: "LOG_FORMAT", Value: matchLogFormat(node.Log.Format)},
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
				{Name: LogsVolume, MountPath: "/var/log"},
				{Name: hostDevVolume, MountPath: "/dev"},
				{Name: hostSysVolume, MountPath: "/sys"},
				{Name: hostRunUdevVolume, MountPath: "/run/udev"},
				{Name: hostRunLVMVolume, MountPath: "/run/lvm"},
				{Name: hostRunLock, MountPath: "/run/lock"},
				{Name: CSISocketDirVolume, MountPath: "/csi"},
				{Name: mountPointDirVolume, MountPath: "/var/lib/kubelet/pods", MountPropagation: &bidirectional},
				{Name: csiPathVolume, MountPath: "/var/lib/kubelet/plugins/kubernetes.io/csi", MountPropagation: &bidirectional},
				{Name: hostRootVolume, MountPath: "/hostroot", MountPropagation: &bidirectional},
			},
		},
		{
			Name:            "drivemgr",
			Image:           constructFullImageName(driveMgr.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(driveMgr.Image.PullPolicy),
			Args:            args,
			Env: []corev1.EnvVar{
				{Name: "LOG_FORMAT", Value: matchLogFormat(node.Log.Format)},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
			},
			SecurityContext: &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			VolumeMounts:    mounts,
		},
	}
}
