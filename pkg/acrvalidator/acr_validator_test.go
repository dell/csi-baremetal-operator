package acrvalidator

import (
	"context"
	"github.com/sirupsen/logrus"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/dell/csi-baremetal-operator/pkg/common"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
)

const (
	testNS = "ns"
)

var (
	ctx = context.Background()
)

func Test_validateACRs(t *testing.T) {
	t.Run("Should not delete ACR if pod exists", func(t *testing.T) {
		var (
			pod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: testNS,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			}
			acr = acrcrd.AvailableCapacityReservation{
				ObjectMeta: metav1.ObjectMeta{
					Name: getReservationName(&pod),
				},
				Spec: api.AvailableCapacityReservation{
					Namespace: testNS,
				},
			}
			updatedPod = corev1.Pod{}
			updatedACR = acrcrd.AvailableCapacityReservation{}
		)

		cv := setupACRValidator(&pod, &acr)
		cv.validateACRs()

		err := cv.Client.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, &updatedPod)
		assert.Nil(t, err)
		assert.NotNil(t, updatedPod)

		err = cv.Client.Get(ctx, client.ObjectKey{Name: acr.Name}, &updatedACR)
		assert.Nil(t, err)
		assert.NotNil(t, updatedACR)
	})

	t.Run("Should delete ACR if pod ready", func(t *testing.T) {
		var (
			pod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: testNS,
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			}
			acr = acrcrd.AvailableCapacityReservation{
				ObjectMeta: metav1.ObjectMeta{
					Name: getReservationName(&pod),
				},
				Spec: api.AvailableCapacityReservation{
					Namespace: testNS,
				},
			}
			updatedPod = corev1.Pod{}
			updatedACR = acrcrd.AvailableCapacityReservation{}
		)

		cv := setupACRValidator(&pod, &acr)
		cv.validateACRs()

		err := cv.Client.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, &updatedPod)
		assert.Nil(t, err)
		assert.NotNil(t, updatedPod)

		err = cv.Client.Get(ctx, client.ObjectKey{Name: acr.Name, Namespace: ""}, &updatedACR)
		assert.NotNil(t, err)
		assert.True(t, k8serrors.IsNotFound(err))
	})

	t.Run("Should delete ACR if pod removed", func(t *testing.T) {
		var (
			pod = corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod1",
					Namespace: testNS,
				},
			}
			acr = acrcrd.AvailableCapacityReservation{
				ObjectMeta: metav1.ObjectMeta{
					Name: getReservationName(&pod),
				},
				Spec: api.AvailableCapacityReservation{
					Namespace: testNS,
				},
			}
			updatedACR = acrcrd.AvailableCapacityReservation{}
		)

		cv := setupACRValidator(&acr)
		cv.validateACRs()

		err := cv.Client.Get(ctx, client.ObjectKey{Name: acr.Name, Namespace: ""}, &updatedACR)
		assert.NotNil(t, err)
		assert.True(t, k8serrors.IsNotFound(err))
	})
}

func setupACRValidator(objects ...client.Object) *ACRValidator {
	scheme, _ := common.PrepareScheme()
	builder := fake.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	client := builderWithScheme.WithObjects(objects...).Build()

	return &ACRValidator{
		Client: client,
		Log:    logrus.New().WithField("component", "ACRValidatorTest"),
	}
}

func getReservationName(pod *corev1.Pod) string {
	namespace := pod.Namespace
	if namespace == "" {
		namespace = "default"
	}

	return namespace + "-" + pod.Name
}
