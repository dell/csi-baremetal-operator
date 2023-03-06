package patcher

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	openshiftv1 "github.com/openshift/api/config/v1"
	oov1 "github.com/openshift/api/operator/v1"
	ssv1 "github.com/openshift/secondary-scheduler-operator/pkg/apis/secondaryscheduler/v1"
	corev1 "k8s.io/api/core/v1"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	openshiftConfigNamespace              = "openshift-config"
	openshiftSchedulerPolicyConfigMapName = "scheduler-policy"

	openshiftPolicyFile = "policy.cfg"

	openshiftSchedulerResourceName = "cluster"
	// OpenshiftSecondarySchedulerLabelValue - Openshift Secondary Scheduler Pod Label Value
	OpenshiftSecondarySchedulerLabelValue = "secondary-scheduler"
	// OpenshiftSecondarySchedulerNamespace - Namespace for Openshift Secondary Scheduler Resources
	OpenshiftSecondarySchedulerNamespace = "openshift-secondary-scheduler-operator"
	openshiftSecondarySchedulerDataKey   = "config.yaml"

	openshiftSecondarySchedulerDefaultImageName = "kube-scheduler"
	openshiftSecondarySchedulerDefaultImageTag  = "v0.23.10"

	csiOpenshiftSecondarySchedulerConfigMapName = "csi-baremetal-scheduler-config"

	csiExtenderName = constant.CSIName + "-se"

	extenderFilterURLFormat = "http://%s:%s%s"
	extenderFilterPattern   = "/filter"

	existing3rdPartySecondarySchedulerErrMsg = "existing 3rd-party secondary scheduler"

	getSchedulerExtenderIPInterval   = 10 * time.Second
	getSchedulerExtenderIPMaxRetires = 12

	selectedSchedulerExtenderIPConfigMapName    = "selected-scheduler-extender-ip"
	selectedSchedulerExtenderIPConfigMapDataKey = "selectedSchedulerExtenderIP"
)

func (p *SchedulerPatcher) checkSchedulerExtender(ip string, port string) error {
	if p.HTTPClient == nil {
		p.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	extenderFilterURL := fmt.Sprintf(extenderFilterURLFormat, ip, port, p.ExtenderPatternChecked)
	request, err := http.NewRequest(http.MethodGet, extenderFilterURL, nil)
	if err != nil {
		return err
	}
	request.Header.Add("Accept", "application/json")
	response, err := p.HTTPClient.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			p.Log.Error("Cannot close response body with error: ", err.Error())
		}
	}()
	if response.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("scheduler extender filter %s doesn't work", extenderFilterURL)
}

func (p *SchedulerPatcher) getSchedulerExtenderIP(ctx context.Context, csi *csibaremetalv1.Deployment,
	scheme *runtime.Scheme) (string, error) {
	extenderPort := csi.Spec.Scheduler.ExtenderPort
	if p.SelectedSchedulerExtenderIP != "" {
		if err := p.checkSchedulerExtender(p.SelectedSchedulerExtenderIP, extenderPort); err != nil {
			p.Log.Warnf("Current selected scheduler extender %s doesn't work: %s",
				p.SelectedSchedulerExtenderIP, err.Error())
		} else {
			return p.SelectedSchedulerExtenderIP, nil
		}
	}

	// Now try to get selected extender IP from selectedSchedulerExtenderIPConfigMap
	cfClient := p.Clientset.CoreV1().ConfigMaps(csi.Namespace)
	selectedSchedulerExtenderIPConfigMap, err := cfClient.Get(ctx, selectedSchedulerExtenderIPConfigMapName,
		metav1.GetOptions{})
	if err == nil {
		selectedSchedulerExtenderIP := selectedSchedulerExtenderIPConfigMap.Data[selectedSchedulerExtenderIPConfigMapDataKey]
		if selectedSchedulerExtenderIP != p.SelectedSchedulerExtenderIP {
			if err = p.checkSchedulerExtender(selectedSchedulerExtenderIP, extenderPort); err != nil {
				p.Log.Warnf("selectedSchedulerExtenderIPConfigMap's IP %s doesn't work: %s",
					selectedSchedulerExtenderIP, err.Error())
			} else {
				p.SelectedSchedulerExtenderIP = selectedSchedulerExtenderIP
				return p.SelectedSchedulerExtenderIP, nil
			}
		}
	} else if !k8sError.IsNotFound(err) {
		p.Log.Warnf("Error in getting selectedSchedulerExtenderIPConfigMap: %s", err.Error())
	}

	// Try to get new scheduler extender IP
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
				if err := p.checkSchedulerExtender(podIP, extenderPort); err != nil {
					p.Log.Warnf("Scheduler extender %s doesn't work: %s", podIP, err.Error())
				} else {
					p.SelectedSchedulerExtenderIP = podIP

					// also need to store this IP in selectedSchedulerExtenderIPConfigMap
					selectedSchedulerExtenderIPConfigMap = createSelectedSchedulerExtenderIPConfigMap(podIP, csi)
					err = controllerutil.SetControllerReference(csi, selectedSchedulerExtenderIPConfigMap, scheme)
					if err != nil {
						p.Log.Warnf("Error in setting selectedSchedulerExtenderIPConfigMap's owner: %s", err.Error())
					}

					err = common.UpdateConfigMap(ctx, p.Clientset, selectedSchedulerExtenderIPConfigMap, p.Log)
					if err != nil {
						p.Log.Warnf("Error in updating selectedSchedulerExtenderIPConfigMap: %s", err.Error())
					}
					return podIP, nil
				}
			}
		}
		return "", fmt.Errorf("no working scheduler extender found")
	}
	return "", fmt.Errorf("no scheduler extender found")
}

func createSelectedSchedulerExtenderIPConfigMap(selectedSchedulerExtenderIP string,
	csi *csibaremetalv1.Deployment) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      selectedSchedulerExtenderIPConfigMapName,
			Namespace: csi.Namespace,
		},
		Data: map[string]string{selectedSchedulerExtenderIPConfigMapDataKey: selectedSchedulerExtenderIP},
	}
}

func (p *SchedulerPatcher) createOpenshiftConfig(ctx context.Context, csi *csibaremetalv1.Deployment,
	useOpenshiftSecondaryScheduler bool, scheme *runtime.Scheme, checkInterval time.Duration, maxRetries int) (string, error) {
	if useOpenshiftSecondaryScheduler {
		// try to get scheduler extender IP
		var (
			selectedSchedulerExtenderIP string
			err                         error
		)

		i := 0
		for ; i < maxRetries; i++ {
			selectedSchedulerExtenderIP, err = p.getSchedulerExtenderIP(ctx, csi, scheme)
			if err == nil {
				break
			}
			p.SelectedSchedulerExtenderIP = ""
			p.Log.Warnf("Fail to get scheduler extender IP: %s", err.Error())
			<-time.After(checkInterval)
		}
		if i == maxRetries {
			return "", err
		}
		p.Log.Infof("Selected Scheduler Extender's IP: %s", selectedSchedulerExtenderIP)

		return fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1beta3
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
    ignorable: true`, selectedSchedulerExtenderIP, csi.Spec.Scheduler.ExtenderPort), nil
	}
	return fmt.Sprintf(`{
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
}`, csi.Spec.Scheduler.ExtenderPort), nil
}

func (p *SchedulerPatcher) patchOpenShift(ctx context.Context, csi *csibaremetalv1.Deployment,
	useOpenshiftSecondaryScheduler bool, scheme *runtime.Scheme) error {
	config, err := p.createOpenshiftConfig(ctx, csi, useOpenshiftSecondaryScheduler, scheme,
		getSchedulerExtenderIPInterval, getSchedulerExtenderIPMaxRetires)
	if err != nil {
		return err
	}

	expected := createOpenshiftConfigMapObject(config, useOpenshiftSecondaryScheduler)

	// TODO csi can't control cm in another namespace https://github.com/dell/csi-baremetal/issues/470
	// if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
	// 	return err
	// }

	err = common.UpdateConfigMap(ctx, p.Clientset, expected, p.Log)
	if err != nil {
		return err
	}

	// try to patch
	if useOpenshiftSecondaryScheduler {
		_, err = p.patchSecondaryScheduler(ctx, csi)
	} else {
		err = p.patchScheduler(ctx, openshiftSchedulerPolicyConfigMapName)
	}
	if err != nil {
		p.Log.Error(err, "Failed to patch Openshift Scheduler")
		return err
	}

	return nil
}

func (p *SchedulerPatcher) unPatchOpenShift(ctx context.Context) error {
	var errMsgs []string

	useOpenshiftSecondaryScheduler, err := p.useOpenshiftSecondaryScheduler(constant.PlatformOpenShift)
	if err != nil {
		return err
	}
	var (
		cmName string
		cmNS   string
	)
	if useOpenshiftSecondaryScheduler {
		cmName = csiOpenshiftSecondarySchedulerConfigMapName
		cmNS = OpenshiftSecondarySchedulerNamespace
	} else {
		cmName = openshiftSchedulerPolicyConfigMapName
		cmNS = openshiftConfigNamespace
	}
	// TODO Remove after https://github.com/dell/csi-baremetal/issues/470
	cfClient := p.Clientset.CoreV1().ConfigMaps(cmNS)
	err = cfClient.Delete(ctx, cmName, metav1.DeleteOptions{})
	if err != nil {
		p.Log.Error(err, "Failed to delete Openshift extender ConfigMap")
		errMsgs = append(errMsgs, err.Error())
	}

	if useOpenshiftSecondaryScheduler {
		err = p.unpatchSecondaryScheduler(ctx)
	} else {
		err = p.unpatchScheduler(ctx, openshiftSchedulerPolicyConfigMapName)
	}
	if err != nil {
		p.Log.Error(err, "Failed to unpatch Scheduler")
		errMsgs = append(errMsgs, err.Error())
	}

	if len(errMsgs) != 0 {
		return fmt.Errorf(strings.Join(errMsgs, "\n"))
	}

	return nil
}

func (p *SchedulerPatcher) retryPatchOpenshift(ctx context.Context, csi *csibaremetalv1.Deployment,
	useOpenshiftSecondaryScheduler bool, scheme *runtime.Scheme) error {
	err := p.unPatchOpenShift(ctx)
	if err != nil {
		p.Log.Error(err, "Failed to unpatch Openshift Scheduler")
		return err
	}

	err = p.patchOpenShift(ctx, csi, useOpenshiftSecondaryScheduler, scheme)
	if err != nil {
		return err
	}

	return nil
}

func createOpenshiftConfigMapObject(config string, useOpenshiftSecondaryScheduler bool) *corev1.ConfigMap {
	var (
		cmName    string
		cmNS      string
		cmDataKey string
	)
	if useOpenshiftSecondaryScheduler {
		cmName = csiOpenshiftSecondarySchedulerConfigMapName
		cmNS = OpenshiftSecondarySchedulerNamespace
		cmDataKey = openshiftSecondarySchedulerDataKey
	} else {
		cmName = openshiftSchedulerPolicyConfigMapName
		cmNS = openshiftConfigNamespace
		cmDataKey = openshiftPolicyFile
	}
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: cmNS,
		},
		Data: map[string]string{cmDataKey: config},
	}
}

func (p *SchedulerPatcher) patchSecondaryScheduler(ctx context.Context, csi *csibaremetalv1.Deployment) (*ssv1.SecondaryScheduler, error) {
	secondaryScheduler := &ssv1.SecondaryScheduler{}

	var csiOpenshiftSecondarySchedulerImage *components.Image
	csiOpenshiftSecondaryScheduler := csi.Spec.Scheduler.OpenshiftSecondaryScheduler
	if csiOpenshiftSecondaryScheduler != nil && csiOpenshiftSecondaryScheduler.Image != nil {
		csiOpenshiftSecondarySchedulerImage = csi.Spec.Scheduler.OpenshiftSecondaryScheduler.Image
		if csiOpenshiftSecondarySchedulerImage.Name == "" || csiOpenshiftSecondarySchedulerImage.Tag == "" {
			p.Log.Warn("Invalid secondary scheduler image provided! Use default secondary scheduler image instead!")
			csiOpenshiftSecondarySchedulerImage = &components.Image{
				Name: openshiftSecondarySchedulerDefaultImageName,
				Tag:  openshiftSecondarySchedulerDefaultImageTag,
			}
		}
	} else {
		csiOpenshiftSecondarySchedulerImage = &components.Image{
			Name: openshiftSecondarySchedulerDefaultImageName,
			Tag:  openshiftSecondarySchedulerDefaultImageTag,
		}
	}
	csiOpenshiftSecondarySchedulerImageURL := common.ConstructFullImageName(csiOpenshiftSecondarySchedulerImage, csi.Spec.GlobalRegistry)

	err := p.Client.Get(ctx, client.ObjectKey{Name: openshiftSchedulerResourceName,
		Namespace: OpenshiftSecondarySchedulerNamespace}, secondaryScheduler)
	switch {
	case err != nil:
		// Fresh install with no existing secondary scheduler
		if k8sError.IsNotFound(err) {
			secondaryScheduler = &ssv1.SecondaryScheduler{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      openshiftSchedulerResourceName,
					Namespace: OpenshiftSecondarySchedulerNamespace,
				},
				Spec: ssv1.SecondarySchedulerSpec{
					OperatorSpec: oov1.OperatorSpec{
						ManagementState:  "Managed",
						OperatorLogLevel: "Normal",
						LogLevel:         "Normal",
					},
					SchedulerConfig: csiOpenshiftSecondarySchedulerConfigMapName,
					SchedulerImage:  csiOpenshiftSecondarySchedulerImageURL,
				},
			}
			err = p.Client.Create(ctx, secondaryScheduler)
			if err != nil {
				return nil, err
			}
			p.Log.Info("SecondaryScheduler CR cluster has been successfully created")
			return secondaryScheduler, nil
		}
		return nil, err
	// Existing 3rd-party secondary scheduler
	case secondaryScheduler.Spec.SchedulerConfig != csiOpenshiftSecondarySchedulerConfigMapName:
		p.Log.Error("Existing 3rd-party secondary scheduler! Baremetal CSI will not be installed!")
		return nil, errors.New(existing3rdPartySecondarySchedulerErrMsg)
	// Existing csi-baremetal secondary scheduler, update scheduler image if necessary
	case secondaryScheduler.Spec.SchedulerImage != csiOpenshiftSecondarySchedulerImageURL:
		secondaryScheduler.Spec.SchedulerImage = csiOpenshiftSecondarySchedulerImageURL
		if err = p.Client.Update(ctx, secondaryScheduler); err != nil {
			return nil, err
		}
		p.Log.Info("Secondary scheduler image has been successfully updated")
		return secondaryScheduler, nil
	}

	return secondaryScheduler, nil
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

func (p *SchedulerPatcher) unpatchSecondaryScheduler(ctx context.Context) error {
	secondaryScheduler := &ssv1.SecondaryScheduler{}

	err := p.Client.Get(ctx, client.ObjectKey{Name: openshiftSchedulerResourceName,
		Namespace: OpenshiftSecondarySchedulerNamespace}, secondaryScheduler)
	if err != nil {
		return err
	}

	// only delete secondaryscheduler CR cluster created by baremetal CSI
	if secondaryScheduler.Spec.SchedulerConfig == csiOpenshiftSecondarySchedulerConfigMapName {
		err = p.Client.Delete(ctx, secondaryScheduler)
		if err != nil {
			return err
		}
		p.Log.Info("SecondaryScheduler CR cluster has been successfully deleted")
	} else {
		p.Log.Info("3rd-party secondary scheduler still exists!")
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
