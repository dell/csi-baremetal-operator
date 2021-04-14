package pkg

import (
	"context"

	openshiftv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	openshiftNS     = "openshift-config"
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

func (p *SchedulerPatcher) PatchOpenShift(ctx context.Context, scheme *runtime.Scheme) error {
	cfClient := p.CoreV1().ConfigMaps(openshiftNS)
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
		p.Logger.Error(err, "Failed to create configmap")
		return err
	}

	err = p.patchSheduler(ctx, openshiftConfig)
	if err != nil {
		p.Logger.Error(err, "Failed to patch Scheduler")
		return err
	}

	return nil
}

func (p *SchedulerPatcher) UnPatchOpenShift(ctx context.Context) error {
	cfClient := p.CoreV1().ConfigMaps(openshiftNS)
	err := cfClient.Delete(openshiftConfig, metav1.NewDeleteOptions(0))
	if err != nil {
		p.Logger.Error(err, "Failed to delete configmap")
		return err
	}
	return p.unpatchSheduler(ctx)
}

func createOpenshiftConfig() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      openshiftConfig,
			Namespace: openshiftNS,
		},
		Data: map[string]string{oshiftpolicyFile: oshiftpolicy},
	}
}

func (p *SchedulerPatcher) patchSheduler(ctx context.Context, config string) error {
	sc := &openshiftv1.Scheduler{}

	err := p.Client.Get(ctx, client.ObjectKey{Name: "cluster"}, sc)
	if err != nil {
		return err
	}

	sc.Spec.Policy.Name = config

	err = p.Client.Update(ctx, sc)
	if err != nil {
		return err
	}

	return nil
}

func (p *SchedulerPatcher) unpatchSheduler(ctx context.Context) error {
	return p.patchSheduler(ctx, "")
}
