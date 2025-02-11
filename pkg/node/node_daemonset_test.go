package node

import (
	"reflect"
	"strconv"
	"testing"

	v1csi "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/masterminds/semver"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	directory         = corev1.HostPathDirectory
	directoryOrCreate = corev1.HostPathDirectoryOrCreate
	configMapMode     = corev1.ConfigMapVolumeSourceDefaultMode
	unset             = corev1.HostPathUnset

	usedVolumes = []corev1.Volume{
		{
			Name: constant.LogsVolume,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: hostDevVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/dev", Type: &directory},
			},
		},
		{
			Name: hostHomeVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/home", Type: &directory},
			},
		},
		{
			Name: hostSysVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/sys", Type: &directory},
			},
		},
		{
			Name: hostRootVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/", Type: &directory},
			},
		},
		{
			Name: hostRunUdevVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/run/udev", Type: &directory},
			},
		},
		{
			Name: hostRunLVMVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/run/lvm", Type: &directory},
			},
		},
		{
			Name: hostRunLock,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/run/lock", Type: &directory},
			},
		},
		{
			Name: constant.CSISocketDirVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/csi-baremetal", Type: &directoryOrCreate},
			},
		},
		{
			Name: registrationDirVolume, VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins_registry/", Type: &directoryOrCreate},
			},
		},
		{
			Name: mountPointDirVolume, VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/pods", Type: &directory},
			},
		},
		{
			Name: csiPathVolume,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/kubernetes.io/csi", Type: &unset},
			},
		},
		{
			Name: nodeConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: nodeConfigMapName},
					DefaultMode:          &configMapMode,
					Optional:             ptr.To(true),
				},
			},
		},
		constant.CrashVolume,
	}

	csiDeployment = v1csi.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-csi",
		},
		Spec: components.DeploymentSpec{
			Driver: &components.Driver{
				Node: &components.Node{
					DriveMgr: &components.DriveMgr{
						Image: &components.Image{
							Name: "drivemgr",
						},
						Endpoint: "endpoint",
					},
					Image: &components.Image{
						Name: "test",
					},
					Log: &components.Log{
						Level: "debug",
					},
					Sidecars: map[string]*components.Sidecar{
						"livenessprobe": {
							Image: &components.Image{
								Name: "livenessprobe",
							},
							Args: &components.Args{
								Timeout:            "60",
								RetryIntervalStart: "20",
								RetryIntervalMax:   "30",
								WorkerThreads:      1,
							},
						},
						"csi-node-driver-registrar": {
							Image: &components.Image{
								Name: "csi-node-driver-registrar",
							},
							Args: &components.Args{
								Timeout:            "60",
								RetryIntervalStart: "20",
								RetryIntervalMax:   "30",
								WorkerThreads:      1,
							},
						},
					},
				},
			},
			Platform:       constant.PlatformOpenShift,
			GlobalRegistry: "asdrepo.isus.emc.com:9042",
			RegistrySecret: "test-registry-secret",
			NodeSelector:   &components.NodeSelector{Key: "key", Value: "value"},
		},
	}

	platform = &PlatformDescription{
		tag:      "",
		labeltag: "default",
		checkVersion: func(version *semver.Version) bool {
			return false
		},
	}

	expectedDaemonSet = &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csi-baremetal-node",
			Namespace: "test-csi",
			Labels:    common.ConstructLabelAppMap(),
		},
		Spec: v1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: common.ConstructSelectorMap("csi-baremetal-node"),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: common.ConstructLabelMap("csi-baremetal-node", "node"),
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(constant.PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Volumes:                       usedVolumes,
					Containers:                    createNodeContainers(&csiDeployment, platform),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: ptr.To(int64(constant.TerminationGracePeriodSeconds)),
					NodeSelector: map[string]string{
						csiDeployment.Spec.NodeSelector.Key:     csiDeployment.Spec.NodeSelector.Value,
						"nodes.csi-baremetal.dell.com/platform": platform.labeltag,
					},
					ServiceAccountName:       csiDeployment.Spec.Driver.Node.ServiceAccount,
					DeprecatedServiceAccount: csiDeployment.Spec.Driver.Node.ServiceAccount,
					SecurityContext:          &corev1.PodSecurityContext{},
					ImagePullSecrets:         common.MakeImagePullSecrets(csiDeployment.Spec.RegistrySecret),
					SchedulerName:            corev1.DefaultSchedulerName,
					HostIPC:                  true,
				},
			},
		},
	}
)

func Test_IsLoppbackMgr_Containers(t *testing.T) {
	t.Run("Check if is loopback-mgr", func(t *testing.T) {

		assert.True(t, isLoopbackMgr("loopbackmgr"))
		assert.False(t, isLoopbackMgr("anothermgr"))
	})
}

func Test_Create_NodeDaemonSet(t *testing.T) {
	t.Run("Check if daemonset is created", func(t *testing.T) {
		daemonSet := createNodeDaemonSet(&csiDeployment, platform)
		assert.NotNil(t, daemonSet)
		if !reflect.DeepEqual(daemonSet, expectedDaemonSet) {
			t.Errorf("Expected daemonset: %v, but got: %v", expectedDaemonSet, daemonSet)
		}
	})
}

func Test_Create_NodeVolumes(t *testing.T) {
	csiDeployment := v1csi.Deployment{
		Spec: components.DeploymentSpec{
			Driver: &components.Driver{
				Node: &components.Node{
					DriveMgr: &components.DriveMgr{
						Image: &components.Image{
							Name: "drivemgr",
						},
						Endpoint: "endpoint",
					},
				},
			},
		},
	}
	t.Run("Check volumes for non loopback mgr", func(t *testing.T) {
		expectedVolumes := usedVolumes
		volumes := createNodeVolumes(&csiDeployment)

		assert.NotNil(t, volumes)
		if !reflect.DeepEqual(volumes, expectedVolumes) {
			t.Errorf("Expected volumes: %v, but got: %v", expectedVolumes, volumes)
		}
	})

	t.Run("Check volumes for loopback mgr", func(t *testing.T) {
		inDeployment := *csiDeployment.DeepCopy()
		inDeployment.Spec.Driver.Node.DriveMgr.Image.Name = "loopbackmgr"

		expectedVolumes := append(usedVolumes, corev1.Volume{
			Name: driveConfigVolume,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: loopbackManagerConfigName},
					DefaultMode:          &configMapMode,
					Optional:             ptr.To(true),
				},
			}})
		volumes := createNodeVolumes(&csiDeployment)

		assert.NotNil(t, volumes)
		if !reflect.DeepEqual(volumes, expectedVolumes) {
			t.Errorf("Expected volumes: %v, but got: %v", expectedVolumes, volumes)
		}
	})
}
