package pkg


import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

var (
	logControllerEntry = logrus.WithField("Test name", "NodeTest")
	testControllerDeployment = v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-csi",
		},
		Spec: components.DeploymentSpec{
			Driver: &components.Driver{
				Controller: &components.Controller{
					Sidecars: map[string]*components.Sidecar{
						"csi-provisioner": {
							Image: &components.Image{
								Name: "provisioner",
							},				
							Args: &components.Args{
								Timeout: "60", 
	 							RetryIntervalStart: "20",
								RetryIntervalMax: "30",
								WorkerThreads: 1,
							},				
						},
						"csi-resizer": {
							Image: &components.Image{
								Name: "resizer",
							},				
							Args: &components.Args{
								Timeout: "60", 
	 							RetryIntervalStart: "20",
								RetryIntervalMax: "30",
								WorkerThreads: 1,
							},				
						},
						"livenessprobe": {
							Image: &components.Image{
								Name: "livenessprobe",
							},				
							Args: &components.Args{
								Timeout: "60", 
	 							RetryIntervalStart: "20",
								RetryIntervalMax: "30",
								WorkerThreads: 1,
							},				
						},
					},
					Image: &components.Image{
						Name: "test",
					},
					Log: &components.Log{
						Level: "debug",
					},
				},
			},
			Platform: constant.PlatformOpenShift,
			GlobalRegistry: "asdrepo.isus.emc.com:9042",
			RegistrySecret: "test-registry-secret",
			},
		}
	
)

func Test_Update_Controller(t *testing.T) {
	t.Run("Update", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testControllerDeployment.DeepCopy()
		) 
		scheme, _ := common.PrepareScheme()
		controller := prepareController(prepareNodeClientSet())
		err := controller.Update(ctx, deployment, scheme)
		assert.Nil(t, err)
	})
}

func prepareController(clientSet kubernetes.Interface) *Controller {
	return &Controller{
		Clientset: clientSet,
		Entry:    logControllerEntry,
	}
}
