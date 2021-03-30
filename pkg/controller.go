package pkg

import (
	"reflect"
	"strconv"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

const (
	controller     = "controller"
	controllerName = CSIName + "-" + controller
	replicasCount  = 1

	controllerRoleKey            = "csi-do"
	controllerServiceAccountName = "csi-controller-sa"

	// ports
	healthPort = 9999

	resizerName     = "csi-resizer"
	provisionerName = "csi-provisioner"
)

type Controller struct {
	kubernetes.Clientset
	logr.Logger
}

func (c *Controller) Update(csi *csibaremetalv1.Deployment) error {
	namespace := GetNamespace(csi)
	dsClient := c.AppsV1().Deployments(namespace)

	isDeployed, err := isDeploymentDeployed(dsClient, controllerName)
	if err != nil {
		c.Logger.Error(err, "Failed to get daemon set")
		return err
	}

	if isDeployed {
		c.Logger.Info("Deployment already deployed")
		if err := c.handleControllerUpgrade(csi); err != nil {
			c.Logger.Info("Failed to upgrade controller: %v", err)
			return err
		}
		return nil
	}

	// create deployment
	deployment := createControllerDeployment(csi)
	if _, err := dsClient.Create(deployment); err != nil {
		c.Logger.Error(err, "Failed to create deployment")
		return err
	}

	c.Logger.Info("Deployment created successfully")
	return nil
}

func (c *Controller) handleControllerUpgrade(csi *csibaremetalv1.Deployment) error {
	c.Logger.Info("Handle controller upgrade")
	dClient := c.AppsV1().Deployments(GetNamespace(csi))
	deployment, err := dClient.Get(controllerName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	uDeployment := createControllerDeployment(csi)
	c.Logger.Info("Updated deployment", "deployment", uDeployment)
	if !reflect.DeepEqual(deployment.Spec, uDeployment.Spec) {
		deployment.Spec = uDeployment.Spec
		c.Logger.Info("Updating...")
		if _, err = dClient.Update(deployment); err != nil {
			return err
		}
	}
	return nil
}

func createControllerDeployment(csi *csibaremetalv1.Deployment) *v1.Deployment {
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerName,
			Namespace: GetNamespace(csi),
		},
		Spec: v1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(replicasCount),
			// selector
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": controllerName, "role": controllerRoleKey},
			},
			// template
			Template: corev1.PodTemplateSpec{
				// labels and annotations
				ObjectMeta: metav1.ObjectMeta{
					// labels
					Labels: map[string]string{
						"app":                    controllerName,
						"app.kubernetes.io/name": CSIName,
						"role":                   controllerRoleKey,
					},
					// integration with monitoring
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   strconv.Itoa(PrometheusPort),
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{Name: LogsVolume, VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						}},
						{Name: CSISocketDirVolume, VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						}},
					},
					Containers:                    createControllerContainers(csi),
					TerminationGracePeriodSeconds: pointer.Int64Ptr(TerminationGracePeriodSeconds),
					NodeSelector:                  csi.Spec.NodeSelectors,
					ServiceAccountName:            controllerServiceAccountName,
				},
			},
		},
	}
}

func createControllerContainers(csi *csibaremetalv1.Deployment) []corev1.Container {
	var (
		provisioner = csi.Spec.Driver.Controller.Sidecars[provisionerName]
		resizer     = csi.Spec.Driver.Controller.Sidecars[resizerName]
		liveness    = csi.Spec.Driver.Controller.Sidecars[livenessProbeSidecar]
		c           = csi.Spec.Driver.Controller
	)
	return []corev1.Container{
		{
			Name:            controller,
			Image:           constructFullImageName(c.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(c.Image.PullPolicy),
			Args: []string{
				"--endpoint=$(CSI_ENDPOINT)",
				"--namespace=$(NAMESPACE)",
				"--extender=true",
				"--loglevel=" + matchLogLevel(c.Log.Level),
				"--healthport=" + strconv.Itoa(healthPort),
				"--metrics-address=:" + strconv.Itoa(PrometheusPort),
				"--metrics-path=/metrics",
			},
			Env: []corev1.EnvVar{
				{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
				}},
				{Name: "CSI_ENDPOINT", Value: "unix:///csi/csi.sock"},
				{Name: "LOG_FORMAT", Value: matchLogFormat(c.Log.Format)},
				{Name: "KUBE_NODE_NAME", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "spec.nodeName"},
				}},
				{Name: "NAMESPACE", ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{APIVersion: "v1", FieldPath: "metadata.namespace"},
				}},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: LogsVolume, MountPath: "/var/log"},
				{Name: CSISocketDirVolume, MountPath: "/csi"},
			},
			Ports: []corev1.ContainerPort{
				{Name: LivenessPort, ContainerPort: 9808, Protocol: corev1.ProtocolTCP},
				{Name: "metrics", ContainerPort: PrometheusPort, Protocol: corev1.ProtocolTCP},
			},
			LivenessProbe: &corev1.Probe{
				Handler: corev1.Handler{HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString(LivenessPort)}},
				InitialDelaySeconds: 300,
				TimeoutSeconds:      3,
				PeriodSeconds:       10,
				FailureThreshold:    5,
			},
			ReadinessProbe: &corev1.Probe{
				Handler: corev1.Handler{Exec: &corev1.ExecAction{Command: []string{
					"/health_probe",
					"-addr=:9999"}}},
				InitialDelaySeconds: 3,
				PeriodSeconds:       10,
				SuccessThreshold:    1,
				FailureThreshold:    15,
			},
		},
		{
			Name:            provisionerName,
			Image:           constructFullImageName(provisioner.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(provisioner.Image.PullPolicy),
			Args: []string{
				"--csi-address=$(ADDRESS)",
				"--v=5",
				"--feature-gates=Topology=true",
				"--extra-create-metadata",
			},
			Env: []corev1.EnvVar{
				{Name: "ADDRESS", Value: "/csi/csi.sock"},
			},
			VolumeMounts: []corev1.VolumeMount{
				{Name: CSISocketDirVolume, MountPath: "/csi"},
			},
		},
		{
			Name:            resizerName,
			Image:           constructFullImageName(resizer.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(resizer.Image.PullPolicy),
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
				{Name: CSISocketDirVolume, MountPath: "/csi"},
			},
		},
		{
			Name:            livenessProbeSidecar,
			Image:           constructFullImageName(liveness.Image, csi.Spec.GlobalRegistry),
			ImagePullPolicy: corev1.PullPolicy(liveness.Image.PullPolicy),
			Args:            []string{"--csi-address=$(ADDRESS)"},
			VolumeMounts: []corev1.VolumeMount{
				{Name: CSISocketDirVolume, MountPath: "/csi"},
			},
		},
	}
}
