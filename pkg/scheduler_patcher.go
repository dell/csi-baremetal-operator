package pkg

import (
	"context"
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
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	platformVanilla   = "vanilla"
	platformRKE       = "rke"
	platformOpenshift = "openshift"
)

const (
	patcherName          = extenderName + "-patcher"
	patcherContainerName = "schedulerpatcher"

	kubernetesManifestsVolume = "kubernetes-manifests"
	kubernetesSchedulerVolume = "kubernetes-scheduler"
	configurationPath         = "/config"
)

type SchedulerPatcher struct {
	ctx context.Context
	kubernetes.Clientset
	logr.Logger
	Client client.Client
}

func (p *SchedulerPatcher) Update(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	if !csi.Spec.Scheduler.Patcher.Enable {
		p.Logger.Info("Patcher disabled - skipping patcher pod creation")
		return nil
	}
	cfg, err := NewPatcherConfiguration(csi)
	if err != nil {
		return err
	}
	expected := cfg.createPatcherDaemonSet()
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	namespace := common.GetNamespace(csi)
	dsClient := p.AppsV1().DaemonSets(namespace)

	found, err := dsClient.Get(p.ctx, patcherName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			if _, err := dsClient.Create(p.ctx, expected, metav1.CreateOptions{}); err != nil {
				p.Logger.Error(err, "Failed to create daemonset")
				return err
			}

			p.Logger.Info("Daemonset created successfully")
			//p.Logger.Info("Daemonset expected %v", expected)
			return nil
		}

		p.Logger.Error(err, "Failed to get daemonset")
		return err
	}

	if common.DaemonsetChanged(expected, found) {
		found.Spec = expected.Spec
		if _, err := dsClient.Update(p.ctx, found, metav1.UpdateOptions{}); err != nil {
			p.Logger.Error(err, "Failed to update daemonset")
			return err
		}

		p.Logger.Info("Daemonset updated successfully")
		return nil
	}

	return nil
}

func (p patcherConfiguration) createPatcherDaemonSet() *v1.DaemonSet {
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      patcherName,
			Namespace: p.ns,
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
					Labels: map[string]string{
						"app": patcherName,
						// release label used by fluentbit to make "release" folder
						"release": patcherName,
					},
				},
				Spec: corev1.PodSpec{
					Containers:                    p.createPatcherContainers(),
					Volumes:                       p.createPatcherVolumes(),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(constant.TerminationGracePeriodSeconds),
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

func (p patcherConfiguration) createPatcherContainers() []corev1.Container {

	return []corev1.Container{
		{
			Name:            patcherContainerName,
			Image:           common.ConstructFullImageName(p.image, p.globalRegistry),
			ImagePullPolicy: corev1.PullPolicy(p.pullPolicy),
			Command: []string{
				"python3",
				"-u",
				"main.py",
			},
			Args: []string{
				"--loglevel=" + common.MatchLogLevel(p.loglevel),
				"--restore",
				"--interval=" + strconv.Itoa(p.interval),
				"--target-config-path=" + p.targetConfig,
				"--target-policy-path=" + p.targetPolicy,
				"--source-config-path=" + p.configFolder + "/" + configFile,
				"--source-policy-path=" + p.configFolder + "/" + policyFile,
				"--source_config_19_path=" + configurationPath + "/" + config19File,
				"--target_config_19_path=" + p.targetConfig19,
				"--backup-path=" + p.schedulerFolder,
				"--platform=" + p.platform,
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: p.configMapName, MountPath: configurationPath, ReadOnly: true},
				{Name: kubernetesSchedulerVolume, MountPath: p.schedulerFolder},
				{Name: kubernetesManifestsVolume, MountPath: p.manifestsFolder},
				constant.CrashMountVolume,
			},
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
		},
	}
}

func (p patcherConfiguration) createPatcherVolumes() []corev1.Volume {
	var (
		schedulerPatcherConfigMapMode = corev1.ConfigMapVolumeSourceDefaultMode
		unset                         = corev1.HostPathUnset
	)
	return []corev1.Volume{
		{Name: p.configMapName, VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: p.configMapName},
				DefaultMode:          &schedulerPatcherConfigMapMode,
				Optional:             pointer.BoolPtr(true),
			},
		}},
		{Name: kubernetesSchedulerVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: p.schedulerFolder, Type: &unset},
		}},
		{Name: kubernetesManifestsVolume, VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{Path: p.manifestsFolder, Type: &unset},
		}},
		constant.CrashVolume,
	}
}
