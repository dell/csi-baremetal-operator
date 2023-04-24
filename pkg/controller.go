package pkg

import (
	"context"
	"fmt"
	"strconv"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	controller     = "controller"
	controllerName = constant.CSIName + "-" + controller
	replicasCount  = 1

	controllerRoleKey            = "csi-do"
	controllerServiceAccountName = "csi-controller-sa"

	// ports
	healthPort = 9999

	// reservation parameters
	fastDelayEnv       = "RESERVATION_FAST_DELAY"
	slowDelayEnv       = "RESERVATION_SLOW_DELAY"
	maxFastAttemptsEnv = "RESERVATION_MAX_FAST_ATTEMPTS"
)

// Controller controls csi-baremetal-controller
type Controller struct {
	Clientset kubernetes.Interface
	*logrus.Entry
}

// Update updates csi-baremetal-controller or creates if not found
func (c *Controller) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// create deployment
	expected := createControllerDeployment(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	if err := common.UpdateDeployment(ctx, c.Clientset, expected, c.Entry); err != nil {
		return err
	}

	return nil
}

func createControllerDeployment(csi *csibaremetalv1.Deployment) *v1.Deployment {
	var (
		selectors = common.ConstructSelectorMap(controllerName)
		labels    = common.ConstructLabelMap(controllerName, controller)
	)

	selectors["role"] = controllerRoleKey
	labels["role"] = controllerRoleKey

	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerName,
			Namespace: csi.GetNamespace(),
			Labels:    common.ConstructLabelAppMap(),
		},
		Spec: v1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(replicasCount),
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: selectors,
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: labels,
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(constant.PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{Name: constant.LogsVolume, VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						}},
						{Name: constant.CSISocketDirVolume, VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						}},
						constant.CrashVolume,
					},
					Containers:                    createControllerContainers(csi),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(constant.TerminationGracePeriodSeconds),
					NodeSelector:                  common.MakeNodeSelectorMap(csi.Spec.NodeSelector),
					ServiceAccountName:            controllerServiceAccountName,
					DeprecatedServiceAccount:      controllerServiceAccountName,
					SecurityContext:               createControllerSecurityContext(csi.Spec.Driver.Controller.SecurityContext),
					ImagePullSecrets:              common.MakeImagePullSecrets(csi.Spec.RegistrySecret),
					SchedulerName:                 corev1.DefaultSchedulerName,
				},
			},
		},
	}
}

func createControllerContainers(csi *csibaremetalv1.Deployment) []corev1.Container {
	var (
		provisioner            = csi.Spec.Driver.Controller.Sidecars[constant.ProvisionerName]
		resizer                = csi.Spec.Driver.Controller.Sidecars[constant.ResizerName]
		liveness               = csi.Spec.Driver.Controller.Sidecars[constant.LivenessProbeName]
		c                      = csi.Spec.Driver.Controller
		argsTimeout            = "10s"
		argsRetryIntervalStart = "1s"
		argsRetryIntervalMax   = "5m"
		argsWorkerThreads      = 2
	)
	if provisioner.Args != nil {
		argsTimeout = provisioner.Args.Timeout
		argsRetryIntervalStart = provisioner.Args.RetryIntervalStart
		argsRetryIntervalMax = provisioner.Args.RetryIntervalMax
		argsWorkerThreads = provisioner.Args.WorkerThreads
	}
	return []corev1.Container{
		{
			Name:            controller,
			Image:           common.ConstructFullImageName(c.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Args: []string{
				"--endpoint=$(CSI_ENDPOINT)",
				"--namespace=$(NAMESPACE)",
				"--extender=true",
				"--loglevel=" + common.MatchLogLevel(c.Log.Level),
				"--healthport=" + strconv.Itoa(healthPort),
				"--metrics-address=:" + strconv.Itoa(constant.PrometheusPort),
				"--metrics-path=/metrics",
				"--sequential-lvg-reservation=" + strconv.FormatBool(csi.Spec.SequentialLVGReservation),
			},
			Env: []corev1.EnvVar{
				{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "status.podIP",
					},
				}},
				{Name: "CSI_ENDPOINT", Value: "unix:///csi/csi.sock"},
				{Name: "LOG_FORMAT", Value: common.MatchLogFormat(c.Log.Format)},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
				{Name: fastDelayEnv, Value: c.FastDelay},
				{Name: slowDelayEnv, Value: c.SlowDelay},
				{Name: maxFastAttemptsEnv, Value: strconv.FormatUint(uint64(c.MaxFastAttempts), 10)},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: constant.LogsVolume, MountPath: "/var/log"},
				{Name: constant.CSISocketDirVolume, MountPath: "/csi"},
				constant.CrashMountVolume,
			},
			Ports: []corev1.ContainerPort{
				{Name: constant.LivenessPort, ContainerPort: 9808, Protocol: corev1.ProtocolTCP},
				{Name: "metrics", ContainerPort: constant.PrometheusPort, Protocol: corev1.ProtocolTCP},
			},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
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
				ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{
					"/health_probe",
					"-addr=:9999"}}},
				InitialDelaySeconds: 3,
				TimeoutSeconds:      1,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    15,
			},
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
			Resources:                common.ConstructResourceRequirements(c.Resources),
		},
		{
			Name:  constant.ProvisionerName,
			Image: common.ConstructFullImageName(provisioner.Image, csi.Spec.GlobalRegistry),
			Args: append(
				[]string{
					// default csi-provisioner args
					"--csi-address=$(ADDRESS)",
					"--extra-create-metadata",
					"--feature-gates=Topology=true",
				},
				[]string{
					// map helm params to csi-provisioner args
					fmt.Sprintf("--timeout=%v", argsTimeout),
					fmt.Sprintf("--retry-interval-start=%v", argsRetryIntervalStart),
					fmt.Sprintf("--retry-interval-max=%v", argsRetryIntervalMax),
					fmt.Sprintf("--worker-threads=%v", argsWorkerThreads),
				}...,
			),
			Env: []corev1.EnvVar{
				{Name: "ADDRESS", Value: "/csi/csi.sock"},
			},
			Resources: common.ConstructResourceRequirements(provisioner.Resources),
			VolumeMounts: []corev1.VolumeMount{
				{Name: constant.CSISocketDirVolume, MountPath: "/csi"},
				constant.CrashMountVolume,
			},
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
			ImagePullPolicy:          corev1.PullPolicy(csi.Spec.PullPolicy),
		},
		{
			Name:            constant.ResizerName,
			Image:           common.ConstructFullImageName(resizer.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Command:         []string{"/csi-resizer"},
			Args: []string{
				"--csi-address=$(ADDRESS)",
				"--v=5",
				"--leader-election",
			},
			Env: []corev1.EnvVar{
				{Name: "ADDRESS", Value: "/csi/csi.sock"},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: constant.CSISocketDirVolume, MountPath: "/csi"},
				constant.CrashMountVolume,
			},
			TerminationMessagePath:   constant.TerminationMessagePath,
			TerminationMessagePolicy: constant.TerminationMessagePolicy,
			Resources:                common.ConstructResourceRequirements(resizer.Resources),
		},
		{
			Name:            constant.LivenessProbeName,
			Image:           common.ConstructFullImageName(liveness.Image, csi.Spec.GlobalRegistry),
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
			Resources:                common.ConstructResourceRequirements(liveness.Resources),
		},
	}
}

func createControllerSecurityContext(ctx *components.SecurityContext) *corev1.PodSecurityContext {
	if ctx == nil || !ctx.Enable {
		return &corev1.PodSecurityContext{}
	}
	return &corev1.PodSecurityContext{
		RunAsNonRoot: ctx.RunAsNonRoot,
		RunAsUser:    ctx.RunAsUser,
	}
}
