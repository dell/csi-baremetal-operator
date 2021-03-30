package pkg

import (
	"strconv"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	livenessProbeTag       = "v2.1.0"
	driverRegistrarTag     = "v1.0.1-gke.0"
)

type Node struct {
	kubernetes.Clientset
	logr.Logger
}

func (n *Node) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// create daemonset
	expected := createNodeDaemonSet(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	namespace := GetNamespace(csi)
	dsClient := n.AppsV1().DaemonSets(namespace)

	found, err := dsClient.Get(nodeName, metav1.GetOptions{})
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
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(TerminationGracePeriodSeconds),
					NodeSelector:                  csi.Spec.NodeSelectors,
					ServiceAccountName:            nodeServiceAccountName,
					DeprecatedServiceAccount:      nodeServiceAccountName,
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 corev1.DefaultSchedulerName,
					HostIPC:                       true,
				},
			},
		},
	}
}

func createNodeVolumes(deployConfig bool) []corev1.Volume {
	directory := corev1.HostPathDirectory
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	unset := corev1.HostPathUnset
	volumes := make([]corev1.Volume, 0, 13)
	volumes = append(volumes,
		corev1.Volume{Name: LogsVolume, VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
		corev1.Volume{Name: hostDevVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &directory},
		}},
		corev1.Volume{Name: hostHomeVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/home", Type: &directory},
		}},
		corev1.Volume{Name: hostSysVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/sys", Type: &directory},
		}},
		corev1.Volume{Name: hostRootVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/", Type: &directory},
		}},
		corev1.Volume{Name: hostRunUdevVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev", Type: &directory},
		}},
		corev1.Volume{Name: hostRunLVMVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/lvm", Type: &directory},
		}},
		corev1.Volume{Name: hostRunLock, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/run/lock", Type: &directory},
		}},
		corev1.Volume{Name: CSISocketDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/csi-baremetal", Type: &directoryOrCreate},
		}},
		corev1.Volume{Name: registrationDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry/", Type: &directoryOrCreate},
		}},
		corev1.Volume{Name: mountPointDirVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/pods", Type: &directory},
		}},
		corev1.Volume{Name: csiPathVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/kubernetes.io/csi", Type: &unset},
		}},
	)

	if deployConfig {
		configMapMode := corev1.ConfigMapVolumeSourceDefaultMode
		volumes = append(volumes, corev1.Volume{
			Name: driveConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: loopbackManagerConfigName},
					DefaultMode:          &configMapMode,
					Optional:             pointer.BoolPtr(true),
				},
			}})
	}
	return volumes
}

// todo split long methods - https://github.com/dell/csi-baremetal/issues/329
func createNodeContainers(csi *csibaremetalv1.Deployment) []corev1.Container {
	var (
		lp            = NewSidecar(livenessProbeSidecar, livenessProbeTag, "IfNotPresent")
		dr            = NewSidecar(driverRegistrarSidecar, driverRegistrarTag, "IfNotPresent")
		bidirectional = corev1.MountPropagationBidirectional
		driveMgr      = csi.Spec.Driver.Node.DriveMgr
		node          = csi.Spec.Driver.Node
		testEnv       = csi.Spec.GlobalRegistry == ""
	)
	mounts := []corev1.VolumeMount{
		{Name: hostDevVolume, MountPath: "/dev"},
		{Name: hostHomeVolume, MountPath: "/host/home"},
	}
	if testEnv {
		mounts = append(mounts, corev1.VolumeMount{Name: driveConfigVolume, MountPath: "/etc/config"})
	}
	return []corev1.Container{
		{
			Name:            lp.Name,
			Image:           constructFullImageName(lp.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(lp.Image.PullPolicy),
			Args:            []string{"--csi-address=/csi/csi.sock"},
			VolumeMounts: []corev1.VolumeMount{
				{Name: CSISocketDirVolume, MountPath: "/csi"},
			},
			TerminationMessagePath:   defaultTerminationMessagePath,
			TerminationMessagePolicy: defaultTerminationMessagePolicy,
		},
		{
			Name:            dr.Name,
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
			TerminationMessagePath:   defaultTerminationMessagePath,
			TerminationMessagePolicy: defaultTerminationMessagePolicy,
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
					Path:   "/healthz",
					Port:   intstr.FromString(LivenessPort),
					Scheme: corev1.URISchemeHTTP}},
				InitialDelaySeconds: 300,
				TimeoutSeconds:      3,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    5,
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{Exec: &corev1.ExecAction{Command: []string{
					"/health_probe",
					"-addr=:9999"}}},
				InitialDelaySeconds: 3,
				TimeoutSeconds:      1,
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
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "status.podIP"},
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
			TerminationMessagePath:   defaultTerminationMessagePath,
			TerminationMessagePolicy: defaultTerminationMessagePolicy,
		},
		{
			Name:            "drivemgr",
			Image:           constructFullImageName(driveMgr.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(driveMgr.Image.PullPolicy),
			Args: []string{
				"--usenodeannotation=" + strconv.FormatBool(csi.Spec.NodeIDAnnotation),
				"--loglevel=" + matchLogLevel(node.Log.Level),
				"--drivemgrendpoint=" + driveMgr.Endpoint,
			},
			Env: []corev1.EnvVar{
				{Name: "LOG_FORMAT", Value: matchLogFormat(node.Log.Format)},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
			},
			SecurityContext:          &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			VolumeMounts:             mounts,
			TerminationMessagePath:   defaultTerminationMessagePath,
			TerminationMessagePolicy: defaultTerminationMessagePolicy,
		},
	}
}
