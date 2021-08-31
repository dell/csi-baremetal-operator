package patcher

import (
	"context"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

// TODO import from csi-baremetal - https://github.com/dell/csi-baremetal/issues/475
const (
	// ExtenderConfigMapName - the configmap, which contains statuses of kube-scheduler restart
	ExtenderConfigMapName = "extender-readiness"
	// ExtenderConfigMapPath - the path to ExtenderConfigMap
	ExtenderConfigMapPath = "/status"
	// ExtenderConfigMapFile - ExtenderConfigMap data key
	ExtenderConfigMapFile = "nodes.yaml"
)

// ExtenderReadinessOptions contains options to deploy ExtenderConfigMap
type ExtenderReadinessOptions struct {
	watchedConfigMapName      string
	watchedConfigMapNamespace string

	readinessConfigMapName      string
	readinessConfigMapNamespace string
	readinessConfigMapFile      string

	kubeSchedulerLabel string
}

// TODO import from csi-baremetal - https://github.com/dell/csi-baremetal/issues/475

// ReadinessStatus contains restart status of one kube-scheduler
type ReadinessStatus struct {
	NodeName      string `yaml:"node_name"`
	KubeScheduler string `yaml:"kube_scheduler"`
	Restarted     bool   `yaml:"restarted"`
}

// ReadinessStatusList contains statuses of all kube-schedulers in cluster
type ReadinessStatusList struct {
	Items []ReadinessStatus `yaml:"nodes"`
}

// NewExtenderReadinessOptions creates ExtenderReadinessOptions
func NewExtenderReadinessOptions(csi *csibaremetalv1.Deployment) (*ExtenderReadinessOptions, error) {
	options := &ExtenderReadinessOptions{}

	switch csi.Spec.Platform {
	case constant.PlatformOpenShift:
		{
			options.watchedConfigMapName = openshiftConfig
			options.watchedConfigMapNamespace = openshiftNS
		}
	case constant.PlatformVanilla, constant.PlatformRKE:
		{
			options.watchedConfigMapName = csi.Spec.Scheduler.Patcher.ConfigMapName
			options.watchedConfigMapNamespace = csi.GetNamespace()
		}
	default:
		{
			return nil, fmt.Errorf("%s platform is not supported platform for the patcher", csi.Spec.Platform)
		}
	}

	options.readinessConfigMapName = ExtenderConfigMapName
	options.readinessConfigMapNamespace = csi.Namespace
	options.readinessConfigMapFile = ExtenderConfigMapFile

	labelKey, labelValue, err := ChooseKubeSchedulerLabel(csi)
	if err != nil {
		return nil, err
	}

	options.kubeSchedulerLabel = fmt.Sprintf("%s=%s", labelKey, labelValue)

	return options, nil
}

// ChooseKubeSchedulerLabel creates a label value and key to find kube-scheduler
func ChooseKubeSchedulerLabel(csi *csibaremetalv1.Deployment) (string, string, error) {
	const (
		OpenshiftKubeSchedulerLabelKey   = "app"
		OpenshiftKubeSchedulerLabelValue = "openshift-kube-scheduler"

		VanillaKubeSchedulerLabelKey   = "component"
		VanillaKubeSchedulerLabelValue = "kube-scheduler"
	)

	switch csi.Spec.Platform {
	case constant.PlatformOpenShift:
		return OpenshiftKubeSchedulerLabelKey, OpenshiftKubeSchedulerLabelValue, nil
	case constant.PlatformVanilla, constant.PlatformRKE:
		return VanillaKubeSchedulerLabelKey, VanillaKubeSchedulerLabelValue, nil
	default:
		return "", "", fmt.Errorf("%s platform is not supported platform for the patcher", csi.Spec.Platform)
	}
}

// IsPatchingEnabled checks enable flag and platform field
// Returns true if patcher enabled and platform is allowed, false otherwise
func IsPatchingEnabled(csi *csibaremetalv1.Deployment) bool {
	spec := csi.Spec
	return spec.Scheduler.Patcher.Enable && isPlatformSupported(spec.Platform)
}

// isPlatformSupported checks for supported platforms
func isPlatformSupported(platform string) bool {
	switch platform {
	case constant.PlatformOpenShift, constant.PlatformVanilla, constant.PlatformRKE:
		return true
	default:
		return false
	}
}

// UpdateReadinessConfigMap collects info about ExtenderReadiness statuses and updates configmap
func (p *SchedulerPatcher) UpdateReadinessConfigMap(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	options, err := NewExtenderReadinessOptions(csi)
	if err != nil {
		return err
	}

	cmCreationTime, err := p.getConfigMapCreationTime(ctx, options)
	if err != nil {
		return err
	}

	readinessStatuses, err := p.updateReadinessStatuses(ctx, options.kubeSchedulerLabel, cmCreationTime)
	if err != nil {
		return err
	}

	expected, err := createReadinessConfigMap(options, readinessStatuses)
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

	// Retry patching procedure if extenders is not ready and
	// 	passed readiness-timeout after configmap creation
	isTimeoutPassed := cmCreationTime.Time.Before(time.Now().Add(time.Minute * time.Duration(-csi.Spec.Scheduler.Patcher.ReadinessTimeout)))
	if !isAllReady(readinessStatuses) && isTimeoutPassed {
		p.Logger.Info("Retry patching")
		switch csi.Spec.Platform {
		case constant.PlatformOpenShift:
			err = p.retryPatchOpenshift(ctx, csi)
			return err
		case constant.PlatformVanilla, constant.PlatformRKE:
			err = p.retryPatchVanilla(ctx, csi, scheme)
			return err
		default:
			return fmt.Errorf("%s platform is not supported platform for the patcher", csi.Spec.Platform)
		}
	}

	return nil
}

func (p *SchedulerPatcher) getConfigMapCreationTime(ctx context.Context, options *ExtenderReadinessOptions) (metav1.Time, error) {
	config, err := p.Clientset.CoreV1().ConfigMaps(options.watchedConfigMapNamespace).Get(ctx, options.watchedConfigMapName, metav1.GetOptions{})
	if err != nil {
		return metav1.Time{}, err
	}

	return config.GetCreationTimestamp(), nil
}

func (p *SchedulerPatcher) updateReadinessStatuses(ctx context.Context, kubeSchedulerLabel string, cmCreationTime metav1.Time) (*ReadinessStatusList, error) {
	readinessStatuses := &ReadinessStatusList{}

	pods, err := p.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{LabelSelector: kubeSchedulerLabel})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		readinessStatus := ReadinessStatus{}
		readinessStatus.KubeScheduler = pod.Name
		readinessStatus.NodeName = pod.Spec.NodeName

		// nolint
		if len(pod.Status.ContainerStatuses) == 0 {
			readinessStatus.Restarted = false
		} else if pod.Status.ContainerStatuses[0].State.Running == nil {
			readinessStatus.Restarted = false
		} else if pod.Status.ContainerStatuses[0].State.Running.StartedAt.Before(&cmCreationTime) {
			readinessStatus.Restarted = false
		} else {
			readinessStatus.Restarted = true
		}

		readinessStatuses.Items = append(readinessStatuses.Items, readinessStatus)
	}

	return readinessStatuses, nil
}

func isAllReady(statuses *ReadinessStatusList) bool {
	for _, status := range statuses.Items {
		if !status.Restarted {
			return false
		}
	}

	return true
}

func createReadinessConfigMap(options *ExtenderReadinessOptions, statuses *ReadinessStatusList) (*corev1.ConfigMap, error) {
	data, err := yaml.Marshal(statuses)
	if err != nil {
		return nil, err
	}

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      options.readinessConfigMapName,
			Namespace: options.readinessConfigMapNamespace,
		},
		Data: map[string]string{options.readinessConfigMapFile: string(data)},
	}, nil
}
