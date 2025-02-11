package pkg

import (
	"context"
	"testing"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
	"github.com/dell/csi-baremetal/pkg/events/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	logEntryDeployment = logrus.New()

	deploymentMatchSecurityContextConstraintsPolicies = []rbacv1.PolicyRule{
		{
			Verbs:         []string{"use"},
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			ResourceNames: []string{"privileged"},
		},
	}

	deploymentMatchPodSecurityPolicyPolicy = rbacv1.PolicyRule{
		Verbs:         []string{"use"},
		APIGroups:     []string{"policy"},
		Resources:     []string{"podsecuritypolicies"},
		ResourceNames: []string{"privileged"},
	}

	deploymentTestRoleBinding = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rolebinding",
			Namespace: "test-csi",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "csi-node-sa",
				Namespace: "test-csi",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     "test-role",
		},
	}

	deploymentTestRolePodSecurityPolicy = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-role",
			Namespace: "test-csi",
		},
		Rules: []rbacv1.PolicyRule{matchPodSecurityPolicyPolicy},
	}

	deployment = csibaremetalv1.Deployment{
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
				Controller: &components.Controller{
					Sidecars: map[string]*components.Sidecar{
						"csi-provisioner": {
							Image: &components.Image{
								Name: "csi-provisioner",
							},
							Args: &components.Args{
								Timeout:            "60",
								RetryIntervalStart: "20",
								RetryIntervalMax:   "30",
								WorkerThreads:      1,
							},
						},
						"csi-resizer": {
							Image: &components.Image{
								Name: "csi-resizer",
							},
							Args: &components.Args{
								Timeout:            "60",
								RetryIntervalStart: "20",
								RetryIntervalMax:   "30",
								WorkerThreads:      1,
							},
						},
						"livenessprobe": {
							Image: &components.Image{
								Name: "livenessprobe",
							},
							Args: &components.Args{
								Timeout:            "60",
								RetryIntervalStart: "20",
								RetryIntervalMax:   "30",
								WorkerThreads:      1,
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
			NodeController: &components.NodeController{
				Enable: true,
				Log: &components.Log{
					Level: "debug",
				},
				Image: &components.Image{
					Name: "test",
				},
				Resources: &components.ResourceRequirements{},
			},
			NodeSelector: &components.NodeSelector{
				Key:   "key",
				Value: "value",
			},
			GlobalRegistry: "asdrepo.isus.emc.com:9042",
			Scheduler: &components.Scheduler{
				PodSecurityPolicy: &components.PodSecurityPolicy{
					Enable:       true,
					ResourceName: "privileged",
				},
				ServiceAccount: "csi-node-sa",
				Patcher: &components.Patcher{
					Enable: true,
				},
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

func Test_NewCSIDeployment(t *testing.T) {
	t.Run("Test creating of the new csi deployment object", func(t *testing.T) {
		var (
			roleBinding = deploymentTestRoleBinding.DeepCopy()
			role        = deploymentTestRolePodSecurityPolicy.DeepCopy()
		)

		scheme, _ := common.PrepareScheme()
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		csiDeployment := NewCSIDeployment(
			prepareFakeNodeClientSet(),
			prepareFakeValidatorClient(scheme, roleBinding, role),
			rbac.NewMatcher(),
			deploymentMatchSecurityContextConstraintsPolicies,
			deploymentMatchPodSecurityPolicyPolicy,
			eventRecorder,
			logEntryDeployment)

		assert.NotNil(t, csiDeployment)
	})
}

func Test_CSIDeployment_Update(t *testing.T) {
	t.Run("Test CSIDeployment Update function", func(t *testing.T) {
		var (
			ctx         = context.Background()
			roleBinding = deploymentTestRoleBinding.DeepCopy()
			role        = deploymentTestRolePodSecurityPolicy.DeepCopy()
		)

		scheme, _ := common.PrepareScheme()
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		csiDeployment := NewCSIDeployment(
			prepareFakeNodeClientSet(),
			prepareFakeValidatorClient(scheme, roleBinding, role),
			rbac.NewMatcher(),
			deploymentMatchSecurityContextConstraintsPolicies,
			deploymentMatchPodSecurityPolicyPolicy,
			eventRecorder,
			logEntryDeployment)

		assert.NotNil(t, csiDeployment)

		err := csiDeployment.Update(ctx, &deployment, scheme)

		assert.Nil(t, err)
	})
}

func Test_CSIDeployment_Uninstall(t *testing.T) {
	t.Run("Test CSIDeployment Uninstall function", func(t *testing.T) {
		var (
			ctx         = context.Background()
			roleBinding = deploymentTestRoleBinding.DeepCopy()
			role        = deploymentTestRolePodSecurityPolicy.DeepCopy()
		)

		scheme, _ := common.PrepareScheme()
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

		csiDeployment := NewCSIDeployment(
			prepareFakeNodeClientSet(),
			prepareFakeValidatorClient(scheme, roleBinding, role),
			rbac.NewMatcher(),
			deploymentMatchSecurityContextConstraintsPolicies,
			deploymentMatchPodSecurityPolicyPolicy,
			eventRecorder,
			logEntryDeployment)

		assert.NotNil(t, csiDeployment)

		err := csiDeployment.Uninstall(ctx, &deployment)

		assert.Nil(t, err)
	})
}

func prepareFakeNodeClientSet(objects ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objects...)
}

func prepareFakeValidatorClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	builder := fakeClient.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	return builderWithScheme.WithObjects(objects...).Build()
}
