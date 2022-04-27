package node

import (
	"context"
	"testing"

	"github.com/dell/csi-baremetal/pkg/events"
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

	v1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/feature/security_verifier"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
)

var (
	nodeSelector *components.NodeSelector

	logEntry = logrus.WithField("Test name", "NodeTest")

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
					Image: &components.Image{},
					DriveMgr: &components.DriveMgr{
						Image: &components.Image{
							Name: "drivemgr",
						},
					},
					Log: &components.Log{
						Level:  components.InfoLevel,
						Format: components.TextFormat,
					},
					Sidecars: map[string]*components.Sidecar{
						constant.LivenessProbeName:   {Image: &components.Image{}},
						constant.DriverRegistrarName: {Image: &components.Image{}},
					},
				},
			},
			Platform: constant.PlatformOpenShift,
		},
	}
)

func TestNewNode(t *testing.T) {
	t.Run("Create Node", func(t *testing.T) {
		// preparing clientset
		clientSet := fake.NewSimpleClientset()
		// preparing client
		scheme, _ := common.PrepareScheme()
		builder := fakeClient.ClientBuilder{}
		builderWithScheme := builder.WithScheme(scheme)
		cl := builderWithScheme.WithObjects().Build()

		node := NewNode(clientSet,
			securityverifier.NewPodSecurityPolicyVerifier(
				validator.NewValidator(rbac.NewValidator(cl, logEntry, rbac.NewMatcher())),
				new(mocks.EventRecorder),
				matchPodSecurityPolicyPolicy,
				logEntry,
			),
			securityverifier.NewSecurityContextConstraintsVerifier(
				validator.NewValidator(rbac.NewValidator(cl, logEntry, rbac.NewMatcher())),
				new(mocks.EventRecorder),
				matchSecurityContextConstraintsPolicies,
				logEntry,
			),
			logEntry,
		)
		assert.NotNil(t, node.clientset)
		assert.NotNil(t, node.log)
		assert.NotNil(t, node.podSecurityPolicyVerifier)
		assert.NotNil(t, node.securityContextConstraintsVerifier)
	})
}

func Test_ValidateRBACSecurityContextConstraints(t *testing.T) {
	t.Run("Not Existing Role and RoleBinding for node ServiceAccount", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeployment.DeepCopy()
		)

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		node := prepareNode(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		err := node.Update(ctx, deployment, scheme)
		assert.Nil(t, err)
	})

	t.Run("Existing Role and RoleBinding for node ServiceAccount", func(t *testing.T) {
		var (
			ctx         = context.Background()
			deployment  = testDeployment.DeepCopy()
			roleBinding = testRoleBinding.DeepCopy()
			role        = testRoleSecurityContextConstraints.DeepCopy()
		)

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		node := prepareNode(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme, roleBinding, role))
		err := node.Update(ctx, deployment, scheme)
		assert.Nil(t, err)
	})

	t.Run("k8s client scheme error during PSP verification", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeployment.DeepCopy()
		)

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(&runtime.Scheme{}))
		err := node.Update(ctx, deployment, &runtime.Scheme{})
		assert.NotNil(t, err)
	})
}

func Test_ValidateRBACPodSecurityPolicy(t *testing.T) {
	t.Run("Not Existing Role and RoleBinding for node ServiceAccount", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeployment.DeepCopy()
		)
		deployment.Spec.Platform = constant.PlatformVanilla

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		node := prepareNode(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		err := node.Update(ctx, deployment, scheme)
		assert.Nil(t, err)
	})

	t.Run("Existing Role and RoleBinding for node ServiceAccount", func(t *testing.T) {
		var (
			ctx         = context.Background()
			deployment  = testDeployment.DeepCopy()
			roleBinding = testRoleBinding.DeepCopy()
			role        = testRolePodSecurityPolicy.DeepCopy()
		)
		deployment.Spec.Platform = constant.PlatformVanilla

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		node := prepareNode(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme, roleBinding, role))
		err := node.Update(ctx, deployment, scheme)
		assert.Nil(t, err)
	})

	t.Run("k8s client scheme error during PSP verification", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeployment.DeepCopy()
		)
		deployment.Spec.Platform = constant.PlatformVanilla

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		node := prepareNode(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(&runtime.Scheme{}))
		err := node.Update(ctx, deployment, &runtime.Scheme{})
		assert.NotNil(t, err)
	})
}

func prepareNodeClientSet(objects ...runtime.Object) kubernetes.Interface {
	return fake.NewSimpleClientset(objects...)
}

func prepareValidatorClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	builder := fakeClient.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	return builderWithScheme.WithObjects(objects...).Build()
}

func prepareNode(eventRecorder events.EventRecorder, clientSet kubernetes.Interface, client client.Client) *Node {
	return NewNode(clientSet,
		securityverifier.NewPodSecurityPolicyVerifier(
			validator.NewValidator(rbac.NewValidator(client, logEntry, rbac.NewMatcher())),
			eventRecorder,
			matchPodSecurityPolicyPolicy,
			logEntry,
		),
		securityverifier.NewSecurityContextConstraintsVerifier(
			validator.NewValidator(rbac.NewValidator(client, logEntry, rbac.NewMatcher())),
			eventRecorder,
			matchSecurityContextConstraintsPolicies,
			logEntry,
		),
		logEntry,
	)
}
