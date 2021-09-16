package patcher

import (
	"context"
	"fmt"
	"strconv"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	patcherName          = constant.CSIName + "-se-patcher"
	patcherContainerName = "schedulerpatcher"

	kubernetesManifestsVolume = "kubernetes-manifests"
	kubernetesSchedulerVolume = "kubernetes-scheduler"
	configurationPath         = "/config"
)

func (p *SchedulerPatcher) updateVanilla(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	err := p.updateVanillaConfigMap(ctx, csi, scheme)
	if err != nil {
		return nil
	}

	err = p.updateVanillaDaemonset(ctx, csi, scheme)
	if err != nil {
		return nil
	}

	return nil
}

func (p *SchedulerPatcher) updateVanillaDaemonset(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	cfg, err := newPatcherConfiguration(csi)
	if err != nil {
		return err
	}

	expected := cfg.createPatcherDaemonSet()
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	if err := common.UpdateDaemonSet(ctx, p.Clientset, expected, p.Logger); err != nil {
		return err
	}

	return nil
}

func (p *SchedulerPatcher) updateVanillaConfigMap(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	expected, err := createVanillaConfig(csi)
	if err != nil {
		return err
	}

	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	err = common.UpdateConfigMap(ctx, p.Clientset, expected, p.Logger)
	if err != nil {
		return err
	}

	return nil
}

func createVanillaConfig(csi *csibaremetalv1.Deployment) (*corev1.ConfigMap, error) {
	cfg, err := newPatcherConfiguration(csi)
	if err != nil {
		return nil, err
	}

	vanillaPolicy :=
		fmt.Sprintf(`apiVersion: v1
kind: Policy
extenders:
  - urlPrefix: "http://127.0.0.1:%s"
    filterVerb: filter
    prioritizeVerb: prioritize
    weight: 1
    #bindVerb: bind
    enableHttps: false
    nodeCacheCapable: false
    ignorable: true
    httpTimeout: 15000000000
`, csi.Spec.Scheduler.ExtenderPort)

	vanillaConfig :=
		fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1alpha1
kind: KubeSchedulerConfiguration
schedulerName: default-scheduler
algorithmSource:
  policy:
    file:
      path: %s
leaderElection:
  leaderElect: true
clientConnection:
  kubeconfig: %s`, cfg.targetPolicy, cfg.kubeconfig)

	vanillaConfig19 :=
		fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1beta1
kind: KubeSchedulerConfiguration
extenders:
  - urlPrefix: "http://127.0.0.1:%s"
    filterVerb: filter
    prioritizeVerb: prioritize
    weight: 1
    #bindVerb: bind
    enableHTTPS: false
    nodeCacheCapable: false
    ignorable: true
    httpTimeout: 15s
leaderElection:
  leaderElect: true
clientConnection:
  kubeconfig: %s`, csi.Spec.Scheduler.ExtenderPort, cfg.kubeconfig)

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      csi.Spec.Scheduler.Patcher.ConfigMapName,
			Namespace: csi.GetNamespace(),
		},
		Data: map[string]string{
			policyFile:   vanillaPolicy,
			configFile:   vanillaConfig,
			config19File: vanillaConfig19,
		}}, nil
}

func (p *SchedulerPatcher) retryPatchVanilla(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	dsClient := p.Clientset.AppsV1().DaemonSets(csi.GetNamespace())
	err := dsClient.Delete(ctx, patcherName, metav1.DeleteOptions{})
	if err != nil {
		p.Logger.Error(err, "Failed to delete patcher daemonset")
		return err
	}

	cmClient := p.Clientset.CoreV1().ConfigMaps(csi.GetNamespace())
	err = cmClient.Delete(ctx, csi.Spec.Scheduler.Patcher.ConfigMapName, metav1.DeleteOptions{})
	if err != nil {
		p.Logger.Error(err, "Failed to delete patcher configmap")
		return err
	}

	err = p.updateVanilla(ctx, csi, scheme)
	if err != nil {
		return err
	}

	return nil
}

func (p patcherConfiguration) createPatcherDaemonSet() *v1.DaemonSet {
	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      patcherName,
			Namespace: p.ns,
			Labels:    common.ConstructLabelAppMap(),
		},
		Spec: v1.DaemonSetSpec{
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: common.ConstructSelectorMap(patcherName),
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: common.ConstructLabelMap(patcherName),
				},
				Spec: corev1.PodSpec{
					Containers:                    p.createPatcherContainers(),
					Volumes:                       p.createPatcherVolumes(),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(constant.TerminationGracePeriodSeconds),
					SecurityContext:               &corev1.PodSecurityContext{},
					ImagePullSecrets:              common.MakeImagePullSecrets(p.registrySecret),
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
