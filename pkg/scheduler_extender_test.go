package pkg


import (
	"context"
	"testing"

	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/dell/csi-baremetal/pkg/events/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
)

var (
	logEntryScheduler = logrus.WithField("Test name", "SchedulerExtenderTest")

	matchSecurityContextConstraintsPolicies = []rbacv1.PolicyRule{
		{
			Verbs:         []string{"use"},
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			ResourceNames: []string{"privileged"},
		},
	}

	matchPodSecurityPolicyPolicy = rbacv1.PolicyRule{
		Verbs:         []string{"use"},
		APIGroups:     []string{"policy"},
		Resources:     []string{"podsecuritypolicies"},
		ResourceNames: []string{"privileged"},
	}

    testRoleBinding = rbacv1.RoleBinding{
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

	testRoleSecurityContextConstraints = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-role",
			Namespace: "test-csi",
		},
		Rules: matchSecurityContextConstraintsPolicies,
	}

	testRolePodSecurityPolicy = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-role",
			Namespace: "test-csi",
		},
		Rules: []rbacv1.PolicyRule{matchPodSecurityPolicyPolicy},
	}

	testDeploymentScheduler = v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-csi",
		},
		Spec: components.DeploymentSpec{
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
			Platform: constant.PlatformVanilla,
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

func Test_Update_Scheduler_Extender(t *testing.T) {
	t.Run("Update", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeploymentScheduler.DeepCopy()
			roleBinding = testRoleBinding.DeepCopy()
			role        = testRolePodSecurityPolicy.DeepCopy()
		) 
		scheme, _ := common.PrepareScheme()
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheduler := prepareSchedulerExtender(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme, roleBinding, role))
		err := scheduler.Update(ctx, deployment, scheme)
		assert.Nil(t, err)
	})
}

func prepareSchedulerExtender(eventRecorder events.EventRecorder, clientSet kubernetes.Interface, client client.Client) *SchedulerExtender {
	return &SchedulerExtender{
		Clientset: clientSet,
		Entry:    logEntryScheduler,
		PodSecurityPolicyVerifier: securityverifier.NewPodSecurityPolicyVerifier(
			validator.NewValidator(rbac.NewValidator(client, logEntryScheduler, rbac.NewMatcher())),
			new(mocks.EventRecorder),
			matchPodSecurityPolicyPolicy,
			logEntryScheduler,
		),
		SecurityContextConstraintsVerifier: securityverifier.NewSecurityContextConstraintsVerifier(
			validator.NewValidator(rbac.NewValidator(client, logEntryScheduler, rbac.NewMatcher())),
			new(mocks.EventRecorder),
			matchSecurityContextConstraintsPolicies,
			logEntryScheduler,
		),
	}
}

func prepareValidatorClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	builder := fakeClient.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	return builderWithScheme.WithObjects(objects...).Build()
}
