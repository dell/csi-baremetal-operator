package node

import (
	"strconv"
	"strings"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	nodeName                  = constant.CSIName + "-node"
	nodeServiceAccountName    = "csi-node-sa"
	loopbackManagerImageName  = "loopbackmgr"
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
)

// GetNodeDaemonsetPodsSelector returns a label-selector to use in the List method
func GetNodeDaemonsetPodsSelector() labels.Selector {
	return labels.SelectorFromSet(common.ConstructSelectorMap(nodeName))
}

func createNodeDaemonSet(csi *csibaremetalv1.Deployment, platform *PlatformDescription) *v1.DaemonSet {
	var nodeSelectors = common.MakeNodeSelectorMap(csi.Spec.NodeSelector)
	nodeSelectors[platformLabel] = platform.labeltag

	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      platform.DaemonsetName(nodeName),
			Namespace: csi.GetNamespace(),
			Labels:    common.ConstructLabelAppMap(),
		},
		Spec: v1.DaemonSetSpec{
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: common.ConstructSelectorMap(nodeName),
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: common.ConstructLabelMap(nodeName),
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(constant.PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Volumes:                       createNodeVolumes(csi),
					Containers:                    createNodeContainers(csi, platform),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(constant.TerminationGracePeriodSeconds),
					NodeSelector:                  nodeSelectors,
					ServiceAccountName:            nodeServiceAccountName,
					DeprecatedServiceAccount:      nodeServiceAccountName,
					SecurityContext:               &corev1.PodSecurityContext{},
					ImagePullSecrets:              common.MakeImagePullSecrets(csi.Spec.RegistrySecret),
					SchedulerName:                 corev1.DefaultSchedulerName,
					HostIPC:                       true,
				},
			},
		},
	}
}

func createNodeVolumes(csi *csibaremetalv1.Deployment) []corev1.Volume {
	directory := corev1.HostPathDirectory
	directoryOrCreate := corev1.HostPathDirectoryOrCreate
	unset := corev1.HostPathUnset
	volumes := make([]corev1.Volume, 0, 14)
	volumes = append(volumes,
		corev1.Volume{Name: constant.LogsVolume, VolumeSource: corev1.VolumeSource{
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
		corev1.Volume{Name: constant.CSISocketDirVolume, VolumeSource: corev1.VolumeSource{
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
		constant.CrashVolume,
	)

	if isLoopbackMgr(csi.Spec.Driver.Node.DriveMgr.Image.Name) {
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

func isLoopbackMgr(imageName string) bool {
	return strings.Contains(imageName, loopbackManagerImageName)
}

// todo split long methods - https://github.com/dell/csi-baremetal/issues/329
func createNodeContainers(csi *csibaremetalv1.Deployment, platform *PlatformDescription) []corev1.Container {
	var (
		bidirectional = corev1.MountPropagationBidirectional
		driveMgr      = csi.Spec.Driver.Node.DriveMgr
		node          = csi.Spec.Driver.Node
		lp            = node.Sidecars[constant.LivenessProbeName]
		dr            = node.Sidecars[constant.DriverRegistrarName]
		nodeImage     = platform.NodeImage(node.Image)
	)
	args := []string{
		"--loglevel=" + common.MatchLogLevel(node.Log.Level),
		"--drivemgrendpoint=" + driveMgr.Endpoint,
	}
	driveMgrMounts := []corev1.VolumeMount{
		{Name: hostDevVolume, MountPath: "/dev"},
		{Name: hostHomeVolume, MountPath: "/host/home"},
		constant.CrashMountVolume,
	}
	if isLoopbackMgr(driveMgr.Image.Name) {
		driveMgrMounts = append(driveMgrMounts, corev1.VolumeMount{Name: driveConfigVolume, MountPath: "/etc/config"})
		args = append(args, "--usenodeannotation="+strconv.FormatBool(csi.Spec.NodeIDAnnotation))
	}
	nodeMounts := []corev1.VolumeMount{
		{Name: constant.LogsVolume, MountPath: "/var/log"},
		{Name: hostDevVolume, MountPath: "/dev"},
		{Name: hostSysVolume, MountPath: "/sys"},
		{Name: hostRunUdevVolume, MountPath: "/run/udev"},
		{Name: hostRunLVMVolume, MountPath: "/run/lvm"},
		{Name: hostRunLock, MountPath: "/run/lock"},
		{Name: constant.CSISocketDirVolume, MountPath: "/csi"},
		{Name: mountPointDirVolume, MountPath: "/var/lib/kubelet/pods", MountPropagation: &bidirectional},
		{Name: csiPathVolume, MountPath: "/var/lib/kubelet/plugins/kubernetes.io/csi", MountPropagation: &bidirectional},
		{Name: hostRootVolume, MountPath: "/hostroot", MountPropagation: &bidirectional},
		constant.CrashMountVolume,
	}
	return []corev1.Container{
		{
			Name:            constant.LivenessProbeName,
			Image:           common.ConstructFullImageName(lp.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Args:            []string{"--csi-address=$(ADDRESS)"},
			Env: []corev1.EnvVar{
				{Name: "ADDRESS", Value: "/csi/csi.sock"},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: constant.CSISocketDirVolume, MountPath: "/csi"},
				constant.CrashMountVolume,
			},
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
		},
		{
			Name:            constant.DriverRegistrarName,
			Image:           common.ConstructFullImageName(dr.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
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
				{Name: constant.CSISocketDirVolume, MountPath: "/csi"},
				{Name: registrationDirVolume, MountPath: "/registration"},
				constant.CrashMountVolume,
			},
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
		},
		{
			Name:            "node",
			Image:           common.ConstructFullImageName(nodeImage, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Args: []string{
				"--csiendpoint=$(CSI_ENDPOINT)",
				"--nodename=$(KUBE_NODE_NAME)",
				"--namespace=$(NAMESPACE)",
				"--extender=true",
				"--loglevel=" + common.MatchLogLevel(node.Log.Level),
				"--metrics-address=:" + strconv.Itoa(constant.PrometheusPort),
				"--metrics-path=/metrics",
				"--drivemgrendpoint=" + driveMgr.Endpoint,
				"--usenodeannotation=" + strconv.FormatBool(csi.Spec.NodeIDAnnotation),
			},
			Ports: []corev1.ContainerPort{
				{Name: constant.LivenessPort, ContainerPort: 9808, Protocol: corev1.ProtocolTCP},
				{Name: "metrics", ContainerPort: constant.PrometheusPort, Protocol: corev1.ProtocolTCP},
			},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromString(constant.LivenessPort),
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
				{Name: "LOG_FORMAT", Value: common.MatchLogFormat(node.Log.Format)},
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
			SecurityContext:          &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			VolumeMounts:             nodeMounts,
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
		},
		{
			Name:            "drivemgr",
			Image:           common.ConstructFullImageName(driveMgr.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Args:            args,
			Env: []corev1.EnvVar{
				{Name: "LOG_FORMAT", Value: common.MatchLogFormat(node.Log.Format)},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
			},
			SecurityContext:          &corev1.SecurityContext{Privileged: pointer.BoolPtr(true)},
			VolumeMounts:             driveMgrMounts,
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
		},
	}
}
