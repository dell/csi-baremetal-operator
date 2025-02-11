package pkg


import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

var (
	logEntry = logrus.WithField("Test name", "NodeTest")
	testDeployment = v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-csi",
		},
		Spec: components.DeploymentSpec{
			Driver: &components.Driver{
				Node: &components.Node{
					ServiceAccount: "csi-node-sa",
					PodSecurityPolicy: &components.PodSecurityPolicy{
						Enable:       true,
						ResourceName: "privileged",
					},
				},
			},
			Platform: constant.PlatformOpenShift,
			GlobalRegistry: "asdrepo.isus.emc.com:9042",
			RegistrySecret: "test-registry-secret",
			NodeController: &components.NodeController{
				Enable: true,
				Log: &components.Log{
					Level: "debug",
				},
				Image: &components.Image{
					Name: "test",
				},
			},
		},
	}
)
func Test_Update_Node_Controller(t *testing.T) {
	t.Run("Update", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeployment.DeepCopy()
		) 
		scheme, _ := common.PrepareScheme()
		node := prepareNode(prepareNodeClientSet())
		err := node.Update(ctx, deployment, scheme)
		assert.Nil(t, err)
	})
}

func prepareNode(clientSet kubernetes.Interface) *NodeController {
	return &NodeController{
		Clientset: clientSet,
		Entry:    logEntry,
	}
}

func prepareNodeClientSet(objects ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objects...)
}
