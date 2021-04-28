package pkg

import (
	"path"
	"strconv"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

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
	configPath    = "/config"

	platformOpenshift = "openshift"
)

type SchedulerPatcher struct {
	kubernetes.Clientset
	logr.Logger
	Client client.Client
}

func (p *SchedulerPatcher) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// create daemonset
	expected := createPatcherDaemonSet(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	namespace := GetNamespace(csi)
	dsClient := p.AppsV1().DaemonSets(namespace)

	found, err := dsClient.Get(patcherName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if _, err := dsClient.Create(expected); err != nil {
				p.Logger.Error(err, "Failed to create daemonset")
				return err
			}

			p.Logger.Info("Daemonset created successfully")
			return nil
		}

		p.Logger.Error(err, "Failed to get daemonset")
		return err
	}

	if daemonsetChanged(expected, found) {
		found.Spec = expected.Spec
		if _, err := dsClient.Update(found); err != nil {
			p.Logger.Error(err, "Failed to update daemonset")
			return err
		}

		p.Logger.Info("Daemonset updated successfully")
		return nil
	}

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
					Containers:                    createPatcherContainers(csi),
					Volumes:                       createPatcherVolumes(),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(TerminationGracePeriodSeconds),
					SecurityContext:               &corev1.PodSecurityContext{},
					SchedulerName:                 corev1.DefaultSchedulerName,
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
	var (
		patcher = csi.Spec.Scheduler.Patcher
	)
	return []corev1.Container{
		{
			Name:            patcherContainerName,
			Image:           constructFullImageName(csi.Spec.Scheduler.Patcher.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Command: []string{
				"python3",
				"-u",
				"main.py",
			},
			Args: []string{
				"--loglevel=" + matchLogLevel(csi.Spec.Scheduler.Log.Level),
				"--restore",
				"--interval=" + strconv.Itoa(patcher.Interval),
				"--manifest=" + path.Join(manifestsPath, "kube-scheduler.yaml"),
				"--target-config-path=" + path.Join(schedulerPath, "config.yaml"),
				"--target-policy-path=" + path.Join(schedulerPath, "policy.yaml"),
				"--source-config-path=" + path.Join(configPath, "config.yaml"),
				"--source-policy-path=" + path.Join(configPath, "policy.yaml"),
				"--source_config_19_path=" + path.Join(configPath, "config-19.yaml"),
				"--target_config_19_path=" + path.Join(schedulerPath, "config-19.yaml"),
				"--backup-path=" + patcher.BackupPath,
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: schedulerPatcherConfigVolume, MountPath: configPath, ReadOnly: true},
				{Name: kubernetesSchedulerVolume, MountPath: schedulerPath},
				{Name: kubernetesManifestsVolume, MountPath: manifestsPath},
			},
			TerminationMessagePath:   defaultTerminationMessagePath,
			TerminationMessagePolicy: defaultTerminationMessagePolicy,
		},
	}
}

func createPatcherVolumes() []corev1.Volume {
	var (
		schedulerPatcherConfigMapMode = corev1.ConfigMapVolumeSourceDefaultMode
		unset                         = corev1.HostPathUnset
	)
	return []corev1.Volume{
		{Name: schedulerPatcherConfigVolume, VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: schedulerPatcherConfigMapName},
				DefaultMode:          &schedulerPatcherConfigMapMode,
				Optional:             pointer.BoolPtr(true),
			},
		}},
		{Name: kubernetesSchedulerVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: schedulerPath, Type: &unset},
		}},
		{Name: kubernetesManifestsVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: manifestsPath, Type: &unset},
		}},
	}
}
