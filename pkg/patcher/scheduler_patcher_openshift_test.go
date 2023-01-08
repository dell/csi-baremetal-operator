package patcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dell/csi-baremetal/pkg/events/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

var (
	csiDeploy = &csibaremetalv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
		},
		Spec: components.DeploymentSpec{
			Platform: constant.PlatformOpenShift,
			Scheduler: &components.Scheduler{
				Patcher: &components.Patcher{
					Enable:        true,
					ConfigMapName: schedulerConf,
				},
				ExtenderPort: "8889",
			},
		},
	}
)

func Test_useOpenshitSecondaryScheduler(t *testing.T) {
	t.Run("Test useOpenshitSecondaryScheduler", func(t *testing.T) {
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		useOpenshiftSecondaryScheduler, err := sp.useOpenshiftSecondaryScheduler(csiDeploy.Spec.Platform)
		assert.NotNil(t, err)

		fakeClient := fakeclientset.NewSimpleClientset()
		fakeDiscovery, ok := fakeClient.Discovery().(*fakediscovery.FakeDiscovery)
		if !ok {
			t.Fatalf("couldn't convert Discovery() to *FakeDiscovery")
		}
		fakeDiscovery.FakedServerVersion = &version.Info{
			Major: "1",
		}
		sp = prepareSchedulerPatcher(eventRecorder, fakeClient, prepareValidatorClient(scheme))
		useOpenshiftSecondaryScheduler, err = sp.useOpenshiftSecondaryScheduler(csiDeploy.Spec.Platform)
		assert.NotNil(t, err)

		fakeDiscovery.FakedServerVersion = &version.Info{
			Major: "1",
			Minor: "25",
		}
		useOpenshiftSecondaryScheduler, err = sp.useOpenshiftSecondaryScheduler(csiDeploy.Spec.Platform)
		assert.Nil(t, err)
		assert.True(t, useOpenshiftSecondaryScheduler)

	})
}

func Test_checkSchedulerExtender(t *testing.T) {
	t.Run("Test checkSchedulerExtender", func(t *testing.T) {
		localExtenderURL := "http://127.0.0.1:8889"

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`OK`))
		}))
		defer server.Close()
		assert.NotNil(t, sp.checkSchedulerExtender(localExtenderURL))

		sp.HTTPClient = server.Client()
		assert.Nil(t, sp.checkSchedulerExtender(server.URL))
		assert.NotNil(t, sp.checkSchedulerExtender("server.URL"))
		assert.NotNil(t, sp.checkSchedulerExtender(localExtenderURL))

		server1 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
			rw.Write(nil)
		}))
		defer server1.Close()
		sp.HTTPClient = server1.Client()
		assert.NotNil(t, sp.checkSchedulerExtender(server1.URL))
	})
}

func Test_PatchOpenshiftSecondaryScheduler(t *testing.T) {
	var (
		ctx         = context.Background()
		curTime     = time.Now()
		podTemplate = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-scheduler",
				Namespace: ns,
				Labels:    map[string]string{"component": "kube-scheduler"},
			},
			Status: corev1.PodStatus{
				ContainerStatuses: []corev1.ContainerStatus{{
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.Time{
								Time: curTime,
							},
						},
					}},
				},
			},
			Spec: corev1.PodSpec{
				NodeName: "node",
			},
		}
	)
	t.Run("Test Patch Openshift Secondary Scheduler with Existing SE IP", func(t *testing.T) {
		pod1 := podTemplate.DeepCopy()
		pod1.Name = pod1.Name + "1"
		pod1.Spec.NodeName = pod1.Spec.NodeName + "1"
		pod1.Status.ContainerStatuses[0].State.Running.StartedAt.Time = curTime.Add(time.Minute)

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		csiDeploy.Spec.Scheduler.Patcher.Enable = false
		assert.Nil(t, sp.Update(ctx, csiDeploy, scheme))
	})
}
