package patcher

import (
	"context"
	"errors"
	"fmt"
	oov1 "github.com/openshift/api/operator/v1"
	ssv1 "github.com/openshift/secondary-scheduler-operator/pkg/apis/secondaryscheduler/v1"
	"strings"

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
)

func (p *SchedulerPatcher) patchOpenShiftSecondaryScheduler(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	secondarySchedulerExtenderConfig := fmt.Sprintf(`apiVersion: kubescheduler.config.k8s.io/v1beta3
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: false
profiles:
  - schedulerName: csi-baremetal-scheduler
extenders:
  - urlPrefix: "http://10.249.232.226:%s"
    filterVerb: filter
    prioritizeVerb: prioritize
    weight: 1
    enableHTTPS: false
    nodeCacheCapable: false
    ignorable: true`, csi.Spec.Scheduler.ExtenderPort)

	expected := createSecondarySchedulerConfig(secondarySchedulerExtenderConfig)

	// TODO csi can't control cm in another namespace https://github.com/dell/csi-baremetal/issues/470
	// if err := controllerutil.SetControllerReference(csi, expected, scheme); err != nil {
	// 	return err
	// }

	err := common.UpdateConfigMap(ctx, p.Clientset, expected, p.Log)
	if err != nil {
		return err
	}

	// try to patch
	err = p.updateSecondaryScheduler(ctx, openshiftConfig)
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
			Name:      "csi-baremetal-scheduler-config",
			Namespace: "openshift-secondary-scheduler-operator",
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

func (p *SchedulerPatcher) updateSecondaryScheduler(ctx context.Context, config string) error {
	secondaryScheduler := &ssv1.SecondaryScheduler{}

	err := p.Client.Get(ctx, client.ObjectKey{Name: "cluster"}, secondaryScheduler)
	if err != nil {
		if k8sError.IsNotFound(err) {
			secondaryScheduler = &ssv1.SecondaryScheduler{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster",
					Namespace: "openshift-secondary-scheduler-operator",
				},
				Spec: ssv1.SecondarySchedulerSpec{
					OperatorSpec: oov1.OperatorSpec{
						ManagementState:  "Managed",
						OperatorLogLevel: "Normal",
						LogLevel:         "Normal",
					},
					SchedulerConfig: "csi-baremetal-scheduler-config",
					SchedulerImage:  "k8s.gcr.io/scheduler-plugins/kube-scheduler:v0.23.10",
				},
			}

			err = p.Client.Create(ctx, secondaryScheduler)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	//name := sc.Spec.Policy.Name
	//// patch when name is not set
	//if name == "" {
	//	sc.Spec.Policy.Name = config
	//	// update scheduler cluster

	//	return nil
	//}
	//// if name is set but not to CSI config name return error
	//if name != config {
	//	return errors.New("scheduler is already patched with the config name: " + name)
	//}

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
