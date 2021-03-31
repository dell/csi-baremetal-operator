package pkg

import (
	"path"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	"github.com/go-logr/logr"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

const (
	patcherName          = extenderName + "-patcher"
	patcherContainerName = "schedulerpatcher"

	// volumes
	schedulerPatcherConfigVolume = "schedulerpatcher-config"
	kubernetesManifestsVolume    = "kubernetes-manifests"
	kubernetesSchedulerVolume    = "kubernetes-scheduler"
	// config maps
	schedulerPatcherConfigMapName = schedulerPatcherConfigVolume

	// paths
	schedulerPath = "/etc/kubernetes/scheduler"
	manifestsPath = "/etc/kubernetes/manifests"
	configPath    = "/conf"
)

type SchedulerPatcher struct {
	kubernetes.Clientset
	logr.Logger
}

func (p *SchedulerPatcher) Update(csi *csibaremetalv1.Deployment) error {
	namespace := GetNamespace(csi)
	dsClient := p.AppsV1().DaemonSets(namespace)

	isDeployed, err := isDaemonSetDeployed(dsClient, patcherName)
	if err != nil {
		p.Logger.Error(err, "Failed to get daemon set")
		return err
	}

	if isDeployed {
		p.Logger.Info("Daemon set already deployed")
		return nil
	}

	// create daemonset
	ds := createPatcherDaemonSet(csi)
	if _, err := dsClient.Create(ds); err != nil {
		p.Logger.Error(err, "Failed to create daemon set")
		return err
	}

	p.Logger.Info("Daemon set created successfully")
	return nil
}

func createPatcherDaemonSet(csi *csibaremetalv1.Deployment) *v1.DaemonSet {
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      patcherName,
			Namespace: GetNamespace(csi),
		},
		Spec: v1.DaemonSetSpec{
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": patcherName},
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: map[string]string{"app": patcherName},
				},
				Spec: corev1.PodSpec{
					Containers: createPatcherContainers(csi),
					Volumes:    createPatcherVolumes(),
					// todo https://github.com/dell/csi-baremetal/issues/329
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

func createPatcherContainers(csi *csibaremetalv1.Deployment) []corev1.Container {
	return []corev1.Container{
		{
			Name:            patcherContainerName,
			Image:           constructFullImageName(csi.Spec.Scheduler.Patcher.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.Scheduler.Patcher.Image.PullPolicy),
			Command: []string{
				"python3",
				"-u",
				"main.py",
			},
			Args: []string{
				"--loglevel=" + matchLogLevel(csi.Spec.Scheduler.Log.Level),
				"--restore",
				"--interval=60",
				"--manifest=" + path.Join(manifestsPath, "kube-scheduler.yaml"),
				"--target-config-path=" + path.Join(schedulerPath, "config.yaml"),
				"--target-policy-path=" + path.Join(schedulerPath, "policy.yaml"),
				"--source-config-path=" + path.Join(configPath, "config.yaml"),
				"--source-policy-path=" + path.Join(configPath, "policy.yaml"),
				"--backup-path=" + schedulerPath,
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: schedulerPatcherConfigVolume, MountPath: configPath, ReadOnly: true},
				{Name: kubernetesSchedulerVolume, MountPath: schedulerPath},
				{Name: kubernetesManifestsVolume, MountPath: manifestsPath},
			},
		},
	}
}

func createPatcherVolumes() []corev1.Volume {
	return []corev1.Volume{
		{Name: schedulerPatcherConfigVolume, VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: schedulerPatcherConfigMapName},
				Optional:             pointer.BoolPtr(true),
			},
		}},
		{Name: kubernetesSchedulerVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: schedulerPath},
		}},
		{Name: kubernetesManifestsVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: manifestsPath},
		}},
	}
}
