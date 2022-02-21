package securityverifier

import (
	"context"
	"errors"
	"testing"

	"github.com/dell/csi-baremetal/pkg/events"
	"github.com/dell/csi-baremetal/pkg/events/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/validator"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
)

var (
	matchPodSecurityPolicyPolicy = rbacv1.PolicyRule{
		Verbs:         []string{"use"},
		APIGroups:     []string{"policy"},
		Resources:     []string{"podsecuritypolicies"},
		ResourceNames: []string{"privileged"},
	}

	logEntry = logrus.WithField("Test name", "NodeTest")

	testServiceAccount = "test-sa"

	testDeployment = v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "test-csi",
		},
	}

	// errors
	arbitraryError        = errors.New("test-error")
	rbacError             = rbac.NewRBACError("test-rbac-error")
	expectedVerifierError = NewVerifierError("Service account has insufficient pod security policies, should have privileged")
)

func Test_HandleError(t *testing.T) {
	t.Run("Handle arbitrary error", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeployment.DeepCopy()
		)
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		verifier := preparePodSecurityPolicyVerifier(eventRecorder, prepareValidatorClient(scheme))
		err := verifier.HandleError(ctx, deployment, testServiceAccount, arbitraryError)
		assert.NotNil(t, err)
		assert.EqualError(t, err, arbitraryError.Error())
	})

	t.Run("Handle RBAC error", func(t *testing.T) {
		var (
			ctx        = context.Background()
			deployment = testDeployment.DeepCopy()
		)
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		verifier := preparePodSecurityPolicyVerifier(eventRecorder, prepareValidatorClient(scheme))
		err := verifier.HandleError(ctx, deployment, testServiceAccount, rbacError)
		assert.NotNil(t, err)
		assert.EqualError(t, err, expectedVerifierError.Error())
	})
}

func prepareValidatorClient(scheme *runtime.Scheme, objects ...client.Object) client.Client {
	builder := fakeClient.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	return builderWithScheme.WithObjects(objects...).Build()
}

func preparePodSecurityPolicyVerifier(eventRecorder events.EventRecorder, client client.Client) SecurityVerifier {
	return NewPodSecurityPolicyVerifier(
		validator.NewValidator(rbac.NewValidator(client, logEntry, rbac.NewMatcher())),
		eventRecorder,
		matchPodSecurityPolicyPolicy,
		logEntry,
	)
}
