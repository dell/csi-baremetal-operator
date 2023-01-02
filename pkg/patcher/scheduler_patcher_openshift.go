package patcher

import (
	"context"
	"errors"
	"fmt"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	oov1 "github.com/openshift/api/operator/v1"
	ssv1 "github.com/openshift/secondary-scheduler-operator/pkg/apis/secondaryscheduler/v1"
	"k8s.io/apimachinery/pkg/labels"
	"net/http"
	"strings"
	"time"

	openshiftv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
)

const (
	openshiftNS     = "openshift-config"
	openshiftConfig = "scheduler-policy"

	openshiftPolicyFile = "policy.cfg"

	openshiftSchedulerResourceName        = "cluster"
	openshiftSecondarySchedulerLabelKey   = "app"
	openshiftSecondarySchedulerLabelValue = "secondary-scheduler"
	openshiftSecondarySchedulerNamespace  = "openshift-secondary-scheduler-operator"

	csiOpenshiftSecondarySchedulerConfig = "csi-baremetal-scheduler-config"
	csiOpenshiftSecondarySchedulerImage  = "k8s.gcr.io/scheduler-plugins/kube-scheduler:v0.23.10"

	csiExtenderName = constant.CSIName + "-se"
)

func (p *SchedulerPatcher) checkSchedulerExtender(ip string, port string) error {
	if p.HttpClient == nil {
		p.HttpClient = &http.Client{Timeout: 5 * time.Second}
	}
	extenderFilterUrl := fmt.Sprintf("http://%s:%s/filter", ip, port)
	request, err := http.NewRequest(http.MethodGet, extenderFilterUrl, nil)
	if err != nil {
		return err
	}
	request.Header.Add("Accept", "application/json")
	response, err := p.HttpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("scheduler extender %s doesn't work", ip)
}

func (p *SchedulerPatcher) getSchedulerExtenderIP(ctx context.Context, extenderPort string) (string, error) {
	if p.SelectedSchedulerExtenderIP != "" {
		if err := p.checkSchedulerExtender(p.SelectedSchedulerExtenderIP, extenderPort); err == nil {
			return p.SelectedSchedulerExtenderIP, nil
		} else {
			p.Log.Warnf("Current Selected Scheduler Extender %s Unworkable: %s",
				p.SelectedSchedulerExtenderIP, err.Error())
		}
	}

	labelSelector := labels.SelectorFromSet(common.ConstructSelectorMap(csiExtenderName))

	var schedulerExtenderPods corev1.PodList
	if err := p.Client.List(ctx, &schedulerExtenderPods, &client.ListOptions{LabelSelector: labelSelector}); err != nil {
		return "", err
	}
	if len(schedulerExtenderPods.Items) > 0 {
		for _, pod := range schedulerExtenderPods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				continue
			}
			podIP := pod.Status.PodIP
			if podIP != "" {
				if err := p.checkSchedulerExtender(podIP, extenderPort); err == nil {
					p.SelectedSchedulerExtenderIP = podIP
					return podIP, nil
				} else {
					p.Log.Warnf("Scheduler Extender %s Unworkable: %s", podIP, err.Error())
				}
			}
		}
		return "", fmt.Errorf("no workable scheduler extender found")
	}
	return "", fmt.Errorf("no scheduler extender found")
}

func (p *SchedulerPatcher) patchOpenShiftSecondaryScheduler(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	schedulerExtenderIP, err := p.getSchedulerExtenderIP(ctx, csi.Spec.Scheduler.ExtenderPort)
	if err != nil {
		return err
	}
	p.Log.Infof("Selected Scheduler Extender's IP: %s", schedulerExtenderIP)

	secondarySchedulerExtenderConfig := fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1beta3
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: false
profiles:
  - schedulerName: csi-baremetal-scheduler
extenders:
  - urlPrefix: "http://%s:%s"
    filterVerb: filter
    prioritizeVerb: prioritize
    weight: 1
    enableHTTPS: false
    nodeCacheCapable: false
    ignorable: true`, schedulerExtenderIP, csi.Spec.Scheduler.ExtenderPort)

	expected := createSecondarySchedulerConfig(secondarySchedulerExtenderConfig)

	// TODO csi can't control cm in another namespace https://github.com/dell/csi-baremetal/issues/470
	// if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
	// 	return err
	// }

	err = common.UpdateConfigMap(ctx, p.Clientset, expected, p.Log)
	if err != nil {
		return err
	}

	// try to patch
	err = p.updateSecondaryScheduler(ctx)
	if err != nil {
		p.Log.Error(err, "Failed to patch Scheduler")
		return err
	}

	return nil
}

func (p *SchedulerPatcher) patchOpenShift(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	openshiftPolicy := fmt.Sprintf(`{
   "kind" : "Policy",
   "apiVersion" : "v1",
   "extenders": [
        {
            "urlPrefix": "http://127.0.0.1:%s",
            "filterVerb": "filter",
            "prioritizeVerb": "prioritize",
            "weight": 1,
            "enableHttps": false,
            "nodeCacheCapable": false,
            "ignorable": true
        }
    ]
}`, csi.Spec.Scheduler.ExtenderPort)

	expected := createOpenshiftConfig(openshiftPolicy)

	// TODO csi can't control cm in another namespace https://github.com/dell/csi-baremetal/issues/470
	// if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
	// 	return err
	// }

	err := common.UpdateConfigMap(ctx, p.Clientset, expected, p.Log)
	if err != nil {
		return err
	}

	// try to patch
	err = p.patchScheduler(ctx, openshiftConfig)
	if err != nil {
		p.Log.Error(err, "Failed to patch Scheduler")
		return err
	}

	return nil
}

func (p *SchedulerPatcher) unPatchOpenShiftSecondaryScheduler(ctx context.Context) error {
	var errMsgs []string

	cfClient := p.Clientset.CoreV1().ConfigMaps(openshiftSecondarySchedulerNamespace)
	err := cfClient.Delete(ctx, csiOpenshiftSecondarySchedulerConfig, metav1.DeleteOptions{})
	if err != nil {
		p.Log.Error(err, "Failed to delete Openshift Secondary Scheduler ConfigMap")
		errMsgs = append(errMsgs, err.Error())
	}

	err = p.uninstallSecondaryScheduler(ctx)
	if err != nil {
		p.Log.Error(err, "Failed to unpatch Scheduler")
		errMsgs = append(errMsgs, err.Error())
	}

	if len(errMsgs) != 0 {
		return fmt.Errorf(strings.Join(errMsgs, "\n"))
	}

	return nil
}

func (p *SchedulerPatcher) unPatchOpenShift(ctx context.Context) error {
	var errMsgs []string

	// TODO Remove after https://github.com/dell/csi-baremetal/issues/470
	cfClient := p.Clientset.CoreV1().ConfigMaps(openshiftNS)
	err := cfClient.Delete(ctx, openshiftConfig, metav1.DeleteOptions{})
	if err != nil {
		p.Log.Error(err, "Failed to delete Openshift extender ConfigMap")
		errMsgs = append(errMsgs, err.Error())
	}

	err = p.unpatchScheduler(ctx, openshiftConfig)
	if err != nil {
		p.Log.Error(err, "Failed to unpatch Scheduler")
		errMsgs = append(errMsgs, err.Error())
	}

	if len(errMsgs) != 0 {
		return fmt.Errorf(strings.Join(errMsgs, "\n"))
	}

	return nil
}

func (p *SchedulerPatcher) retryPatchOpenshift(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	err := p.unPatchOpenShift(ctx)
	if err != nil {
		p.Log.Error(err, "Failed to unpatch Openshift Scheduler")
		return err
	}

	err = p.patchOpenShift(ctx, csi)
	if err != nil {
		return err
	}

	return nil
}

func createSecondarySchedulerConfig(config string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      csiOpenshiftSecondarySchedulerConfig,
			Namespace: openshiftSecondarySchedulerNamespace,
		},
		Data: map[string]string{"config.yaml": config},
	}
}

func createOpenshiftConfig(policy string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      openshiftConfig,
			Namespace: openshiftNS,
		},
		Data: map[string]string{openshiftPolicyFile: policy},
	}
}

func (p *SchedulerPatcher) updateSecondaryScheduler(ctx context.Context) error {
	// TODO make scheduler image version dependent on platform's k8s version
	newSecondaryScheduler := &ssv1.SecondaryScheduler{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      openshiftSchedulerResourceName,
			Namespace: openshiftSecondarySchedulerNamespace,
		},
		Spec: ssv1.SecondarySchedulerSpec{
			OperatorSpec: oov1.OperatorSpec{
				ManagementState:  "Managed",
				OperatorLogLevel: "Normal",
				LogLevel:         "Normal",
			},
			SchedulerConfig: csiOpenshiftSecondarySchedulerConfig,
			SchedulerImage:  csiOpenshiftSecondarySchedulerImage,
		},
	}

	existingSecondaryScheduler := &ssv1.SecondaryScheduler{}
	err := p.Client.Get(ctx, client.ObjectKey{Name: openshiftSchedulerResourceName,
		Namespace: openshiftSecondarySchedulerNamespace}, existingSecondaryScheduler)
	if err != nil {
		if k8sError.IsNotFound(err) {
			err = p.Client.Create(ctx, newSecondaryScheduler)
			if err != nil {
				return err
			}
		}
		return err
	} else if existingSecondaryScheduler.Spec.SchedulerConfig != csiOpenshiftSecondarySchedulerConfig ||
		existingSecondaryScheduler.Spec.SchedulerImage != csiOpenshiftSecondarySchedulerImage {
		if err = p.Client.Update(ctx, newSecondaryScheduler); err != nil {
			return err
		}
	}

	return nil
}

func (p *SchedulerPatcher) patchScheduler(ctx context.Context, config string) error {
	sc := &openshiftv1.Scheduler{}

	err := p.Client.Get(ctx, client.ObjectKey{Name: "cluster"}, sc)
	if err != nil {
		return err
	}

	name := sc.Spec.Policy.Name
	// patch when name is not set
	if name == "" {
		sc.Spec.Policy.Name = config
		// update scheduler cluster
		err = p.Client.Update(ctx, sc)
		if err != nil {
			return err
		}
		return nil
	}
	// if name is set but not to CSI config name return error
	if name != config {
		return errors.New("scheduler is already patched with the config name: " + name)
	}

	return nil
}

func (p *SchedulerPatcher) uninstallSecondaryScheduler(ctx context.Context) error {
	secondaryScheduler := &ssv1.SecondaryScheduler{}

	err := p.Client.Get(ctx, client.ObjectKey{Name: openshiftSchedulerResourceName,
		Namespace: openshiftSecondarySchedulerNamespace}, secondaryScheduler)
	if err != nil {
		return err
	}

	err = p.Client.Delete(ctx, secondaryScheduler)
	if err != nil {
		return err
	}
	return nil
}

func (p *SchedulerPatcher) unpatchScheduler(ctx context.Context, config string) error {
	sc := &openshiftv1.Scheduler{}

	err := p.Client.Get(ctx, client.ObjectKey{Name: "cluster"}, sc)
	if err != nil {
		return err
	}

	name := sc.Spec.Policy.Name
	// patch when name is not set
	switch name {
	case "":
		// already unpatched
		return nil
	case config:
		sc.Spec.Policy.Name = ""
		// update scheduler cluster
		err = p.Client.Update(ctx, sc)
		if err != nil {
			return err
		}
		return nil
	default:
		return errors.New("scheduler was patched with the config name: " + name)
	}
}
