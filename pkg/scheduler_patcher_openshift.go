package pkg

import (
	"context"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	openshiftv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	opeshiftNS      = "openshift-config"
	openshiftConfig = "scheduler-policy"

	oshiftpolicyFile = "policy.cfg"
	oshiftpolicy     = `{
   "kind" : "Policy",
   "apiVersion" : "v1",
   "extenders": [
        {
            "urlPrefix": "http://127.0.0.1:$PORT",
            "filterVerb": "filter",
            "enableHttps": false,
            "nodeCacheCapable": false,
            "ignorable": true
        }
    ]
}`
)

func (p *SchedulerPatcher) UpdateOpenShift(csi *csibaremetalv1.Deployment, scheme *runtime.Scheme) error {
	cfClient := p.CoreV1().ConfigMaps(opeshiftNS)

	oscf, err := cfClient.Get(openshiftConfig, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			p.Logger.Error(err, "Failed to get configmap")
			return err
		}
	} else {
		if v, ok := oscf.Data[oshiftpolicyFile]; ok {
			if v == oshiftpolicy {
				p.Logger.Info("Configmap is already patched")
				return nil
			}
		}

		err := cfClient.Delete(openshiftConfig, metav1.NewDeleteOptions(0))
		if err != nil {
			p.Logger.Error(err, "Failed to delete configmap")
			return err
		}
	}

	_, err = cfClient.Create(createOpenshiftConfig())
	if err != nil {
		p.Logger.Error(err, "Failed to get create configmap")
		return err
	}

	err = p.patchSheduler(openshiftConfig)
	if err != nil {
		p.Logger.Error(err, "Failed to patch Scheduler")
		return err
	}

	return nil
}

func (p *SchedulerPatcher) UnPatchOpenShift() error {
	cfClient := p.CoreV1().ConfigMaps(opeshiftNS)
	err := cfClient.Delete(openshiftConfig, metav1.NewDeleteOptions(0))
	if err != nil {
		p.Logger.Error(err, "Failed to delete configmap")
		return err
	}
	return p.unpatchSheduler()
}

func createOpenshiftConfig() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Data:       map[string]string{oshiftpolicyFile: oshiftpolicy},
	}
}

func (p *SchedulerPatcher) patchSheduler(config string) error {
	sc := &openshiftv1.Scheduler{}

	err := p.Client.Get(context.TODO(), client.ObjectKey{Name: "cluster"}, sc)
	if err != nil {
		return err
	}

	sc.Spec.Policy.Name = config

	err = p.Client.Update(context.TODO(), sc)
	if err != nil {
		return err
	}

	return nil
}

func (p *SchedulerPatcher) unpatchSheduler() error {
	return p.patchSheduler("")
}
