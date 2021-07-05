package pkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	openshiftv1 "github.com/openshift/api/config/v1"
)

const (
	openshiftNS     = "openshift-config"
	openshiftConfig = "scheduler-policy"

	openshiftPolicyFile = "policy.cfg"
)

func (p *SchedulerPatcher) PatchOpenShift(ctx context.Context, csi *csibaremetalv1.Deployment) error {
	openshiftPolicy := fmt.Sprintf(`{
   "kind" : "Policy",
   "apiVersion" : "v1",
   "extenders": [
        {
            "urlPrefix": "http://127.0.0.1:%s",
            "filterVerb": "filter",
            "enableHttps": false,
            "nodeCacheCapable": false,
            "ignorable": true
        }
    ]
}`, csi.Spec.Scheduler.ExtenderPort)

	cfClient := p.CoreV1().ConfigMaps(openshiftNS)
	config, err := cfClient.Get(ctx, openshiftConfig, metav1.GetOptions{})
	// exclude not found error
	if err != nil && !apiErrors.IsNotFound(err) {
		p.Logger.Error(err, "Failed to get ConfigMap")
		return err
	}

	// ConfigMap not found - create
	if apiErrors.IsNotFound(err) {
		_, err = cfClient.Create(ctx, createOpenshiftConfig(openshiftPolicy), metav1.CreateOptions{})
		if err != nil {
			p.Logger.Error(err, "Failed to create ConfigMap")
			return err
		}
	} else {
		// check if already patched and update otherwise
		if v, ok := config.Data[openshiftPolicyFile]; ok && v == openshiftPolicy {
			p.Logger.Info("ConfigMap is already patched")
		} else {
			// try to update
			_, err = cfClient.Update(ctx, createOpenshiftConfig(openshiftPolicy), metav1.UpdateOptions{})
			if err != nil {
				p.Logger.Error(err, "Failed to update ConfigMap")
				return err
			}
		}
	}

	// try to patch
	err = p.patchScheduler(ctx, openshiftConfig)
	if err != nil {
		p.Logger.Error(err, "Failed to patch Scheduler")
		return err
	}

	return nil
}

func (p *SchedulerPatcher) UnPatchOpenShift(ctx context.Context) error {
	cfClient := p.CoreV1().ConfigMaps(openshiftNS)
	// delete ConfigMap
	var errMsgs []string
	err := cfClient.Delete(ctx, openshiftConfig, *metav1.NewDeleteOptions(0))
	if err != nil {
		p.Logger.Error(err, "Failed to delete ConfigMap")
		errMsgs = append(errMsgs, err.Error())
	}
	// Unpatch Scheduler
	err = p.unpatchScheduler(ctx, openshiftConfig)
	if err != nil {
		p.Logger.Error(err, "Failed to unpatch Scheduler")
		errMsgs = append(errMsgs, err.Error())
	}
	// check for errors
	if len(errMsgs) == 0 {
		return nil
	}
	// return errors
	return fmt.Errorf(strings.Join(errMsgs, "\n"))
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
