package pkg

import (
	"context"
	"errors"
	"strconv"

	"github.com/dell/csi-baremetal/pkg/eventing"
	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/patcher"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	"github.com/dell/csi-baremetal-operator/pkg/validator/models"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
	rbacmodels "github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

const (
	extenderContainerName = "scheduler-extender"
	extenderName          = constant.CSIName + "-se"

	extenderPort = 8889
)

// SchedulerExtender controls csi-baremetal-se
type SchedulerExtender struct {
	Clientset kubernetes.Interface
	*logrus.Entry
	Validator     validator.Validator
	EventRecorder events.EventRecorder
	MatchPolicies []rbacv1.PolicyRule
}

// Update updates csi-baremetal-se or creates if not found
func (n *SchedulerExtender) Update(ctx context.Context, csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	// in case of Openshift deployment and non default namespace - validate extender service accounts security bindings
	if csi.Spec.Platform == constant.PlatformOpenShift && csi.Namespace != constant.DefaultNamespace {
		var rbacError rbac.Error
		if err := n.Validator.ValidateRBAC(ctx, &models.RBACRules{
			Data: &rbacmodels.ServiceAccountIsRoleBoundData{
				ServiceAccountName: csi.Spec.Scheduler.ServiceAccount,
				Namespace:          csi.Namespace,
				Role: &rbacv1.Role{
					Rules: n.MatchPolicies,
				},
			},
			Type: models.ServiceAccountIsRoleBound,
		}); err != nil {
			if errors.As(err, &rbacError) {
				n.EventRecorder.Eventf(csi, eventing.WarningType, "ExtenderRoleValidationFailed",
					"ServiceAccount %s has insufficient securityContextConstraints, should have privileged",
					csi.Spec.Scheduler.ServiceAccount)
				n.Warn(rbacError, "Extender service account has insufficient securityContextConstraints, should have privileged")
				return nil
			}
			n.Error(err, "Error occurred while validating extender service account security context bindings")
			return err
		}
	}

	// create daemonset
	expected := n.createExtenderDaemonSet(csi)
	if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
		return err
	}

	if err := common.UpdateDaemonSet(ctx, n.Clientset, expected, n.Entry); err != nil {
		return err
	}

	return nil
}

func (n *SchedulerExtender) createExtenderDaemonSet(csi *csibaremetalv1.Deployment) *v1.DaemonSet {
	var (
		extenderConfigMapMode = corev1.ConfigMapVolumeSourceDefaultMode
		volumes               = []corev1.Volume{constant.CrashVolume}
		isPatchingEnabled     = patcher.IsPatchingEnabled(csi)
	)

	if isPatchingEnabled {
		volumes = append(volumes, corev1.Volume{
			Name: patcher.ExtenderConfigMapName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: patcher.ExtenderConfigMapName},
					DefaultMode:          &extenderConfigMapMode,
					Optional:             pointer.BoolPtr(true),
				},
			}})
	}

	return &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      extenderName,
			Namespace: csi.GetNamespace(),
			Labels:    common.ConstructLabelAppMap(),
		},
		Spec: v1.DaemonSetSpec{
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: common.ConstructSelectorMap(extenderName),
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: common.ConstructLabelMap(extenderName),
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(constant.PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Containers:                    createExtenderContainers(csi, isPatchingEnabled),
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(constant.TerminationGracePeriodSeconds),
					ServiceAccountName:            csi.Spec.Scheduler.ServiceAccount,
					DeprecatedServiceAccount:      csi.Spec.Scheduler.ServiceAccount,
					SecurityContext:               &corev1.PodSecurityContext{},
					ImagePullSecrets:              common.MakeImagePullSecrets(csi.Spec.RegistrySecret),
					SchedulerName:                 corev1.DefaultSchedulerName,
					HostNetwork:                   true,
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
					Volumes: volumes,
				},
			},
		},
	}
}

func createExtenderContainers(csi *csibaremetalv1.Deployment, isPatchingEnabled bool) []corev1.Container {
	volumeMounts := []corev1.VolumeMount{constant.CrashMountVolume}

	if isPatchingEnabled {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      patcher.ExtenderConfigMapName,
			MountPath: patcher.ExtenderConfigMapPath,
			ReadOnly:  true})
	}

	return []corev1.Container{
		{
			Name:            extenderContainerName,
			Image:           common.ConstructFullImageName(csi.Spec.Scheduler.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(csi.Spec.PullPolicy),
			Args: []string{
				"--namespace=$(NAMESPACE)",
				"--provisioner=" + constant.CSIName,
				"--port=" + strconv.Itoa(extenderPort),
				"--healthport=" + strconv.Itoa(healthPort),
				"--loglevel=" + common.MatchLogLevel(csi.Spec.Scheduler.Log.Level),
				"--certFile=",
				"--privateKeyFile=",
				"--metrics-address=:" + strconv.Itoa(constant.PrometheusPort),
				"--metrics-path=/metrics",
				"--usenodeannotation=" + strconv.FormatBool(csi.Spec.NodeIDAnnotation),
				"--isPatchingEnabled=" + strconv.FormatBool(isPatchingEnabled),
			},
			Env: []corev1.EnvVar{
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
				{Name: "LOG_FORMAT", Value: common.MatchLogFormat(csi.Spec.Scheduler.Log.Format)},
			},
			Ports: []corev1.ContainerPort{
				{Name: "metrics", HostPort: constant.PrometheusPort, ContainerPort: constant.PrometheusPort, Protocol: corev1.ProtocolTCP},
				{Name: "extender", HostPort: extenderPort, ContainerPort: extenderPort, Protocol: corev1.ProtocolTCP},
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{Exec: &corev1.ExecAction{Command: []string{
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
			VolumeMounts:             volumeMounts,
			Resources:                common.ConstructResourceRequirements(csi.Spec.Scheduler.Resources),
		},
	}
}
