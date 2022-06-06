package acr

import (
	"context"
	"testing"

	api "github.com/dell/csi-baremetal/api/generated/v1"
	acrcrd "github.com/dell/csi-baremetal/api/v1/acreservationcrd"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/dell/csi-baremetal-operator/pkg/common"
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

		cl := setupClient(&pod, &acr)
		NewACRCleaner(cl, logrus.New().WithField("component", "ACRValidatorTest")).Handle(ctx)

		err := cl.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, &updatedPod)
		assert.Nil(t, err)
		assert.NotNil(t, updatedPod)

		err = cl.Get(ctx, client.ObjectKey{Name: acr.Name}, &updatedACR)
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

		cl := setupClient(&pod, &acr)
		NewACRCleaner(cl, logrus.New().WithField("component", "ACRValidatorTest")).Handle(ctx)

		err := cl.Get(ctx, client.ObjectKey{Name: pod.Name, Namespace: pod.Namespace}, &updatedPod)
		assert.Nil(t, err)
		assert.NotNil(t, updatedPod)

		err = cl.Get(ctx, client.ObjectKey{Name: acr.Name, Namespace: ""}, &updatedACR)
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

		cl := setupClient(&acr)
		NewACRCleaner(cl, logrus.New().WithField("component", "ACRValidatorTest")).Handle(ctx)

		err := cl.Get(ctx, client.ObjectKey{Name: acr.Name, Namespace: ""}, &updatedACR)
		assert.NotNil(t, err)
		assert.True(t, k8serrors.IsNotFound(err))
	})
}

func setupClient(objects ...client.Object) client.Client {
	scheme, _ := common.PrepareScheme()
	builder := fake.ClientBuilder{}
	builderWithScheme := builder.WithScheme(scheme)
	return builderWithScheme.WithObjects(objects...).Build()
}

func getReservationName(pod *corev1.Pod) string {
	namespace := pod.Namespace
	if namespace == "" {
		namespace = "default"
	}

	return namespace + "-" + pod.Name
}
