package patcher


import (
	"context"
	"testing"
	"strings"

	"github.com/dell/csi-baremetal/pkg/events/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
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
					Image: &components.Image{
						Name: "test",
					},
					ConfigMapName: "scheduler-conf",
					Interval:          10,
					RestoreOnShutdown: true,
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

func Test_Update_Retry_Scheduler_Patcher_Vanilla(t *testing.T) {
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
		schedulerPatcher := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme, roleBinding, role))
		err := schedulerPatcher.updateVanilla(ctx, deployment, scheme)
		assert.Nil(t, err)
		err = schedulerPatcher.retryPatchVanilla(ctx, deployment, scheme)
		assert.Nil(t, err)
	})
}

func Test_Retry_Patch_Vanilla_Error(t *testing.T) {
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
		schedulerPatcher := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme, roleBinding, role))
		err := schedulerPatcher.retryPatchVanilla(ctx, deployment, scheme)
		assert.NotNil(t, err)
		assert.True(t, strings.HasSuffix(err.Error(), "not found"))
	})
}