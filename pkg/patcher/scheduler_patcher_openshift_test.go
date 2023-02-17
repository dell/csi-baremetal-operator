package patcher

import (
	"context"
	k8sError "k8s.io/apimachinery/pkg/api/errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
			GlobalRegistry: "asdrepo.isus.emc.com:9042",
			Platform:       constant.PlatformOpenShift,
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
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`OK`))
		}))
		defer server.Close()
		assert.NotNil(t, sp.checkSchedulerExtender("big horse", "-2"))

		sp.HTTPClient = server.Client()

		u, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("Error in parsing server.URL %s", err.Error())
		}
		assert.Nil(t, sp.checkSchedulerExtender(u.Hostname(), u.Port()))
		assert.NotNil(t, sp.checkSchedulerExtender("big", "31"))

		server1 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusNotFound)
			rw.Write(nil)
		}))
		defer server1.Close()
		sp.HTTPClient = server1.Client()
		u, err = url.Parse(server1.URL)
		if err != nil {
			t.Fatalf("Error in parsing server1.URL %s", err.Error())
		}
		assert.NotNil(t, sp.checkSchedulerExtender(u.Hostname(), u.Port()))
	})
}

func Test_getSchedulerExtenderIP(t *testing.T) {
	t.Run("Test getSchedulerExtenderIP", func(t *testing.T) {
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`OK`))
		}))
		defer server.Close()
		u, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("Error in parsing server.URL %s", err.Error())
		}
		csiDeploy.Spec.Scheduler.ExtenderPort = u.Port()
		ctx := context.Background()

		podTemplate := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "csi-baremetal-se-",
				Namespace: ns,
				Labels:    map[string]string{"name": "csi-baremetal-se"},
			},
			Status: corev1.PodStatus{},
			Spec:   corev1.PodSpec{},
		}
		var (
			pendingPod    = podTemplate.DeepCopy()
			workablePod   = podTemplate.DeepCopy()
			unworkablePod = podTemplate.DeepCopy()
		)
		podTemplate.Name = podTemplate.Name + "blank"

		pendingPod.Name = pendingPod.Name + "pending"
		pendingPod.Status.Phase = corev1.PodPending

		workablePod.Name = pendingPod.Name + "workable"
		workablePod.Status.Phase = corev1.PodRunning
		workablePod.Status.PodIP = u.Hostname()

		unworkablePod.Name = unworkablePod.Name + "unworkable"
		unworkablePod.Status.Phase = corev1.PodRunning
		unworkablePod.Status.PodIP = "192.168.1.1"

		// case of no scheduler extender found
		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		sp.HTTPClient = server.Client()
		extenderIP, err := sp.getSchedulerExtenderIP(ctx, u.Port())
		assert.NotNil(t, err)
		assert.Empty(t, extenderIP)

		// case of no workable scheduler extender found
		sp = prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(podTemplate, pendingPod, unworkablePod),
			prepareValidatorClient(scheme, podTemplate, pendingPod, unworkablePod))
		sp.HTTPClient = server.Client()
		extenderIP, err = sp.getSchedulerExtenderIP(ctx, u.Port())
		assert.NotNil(t, err)
		assert.Empty(t, extenderIP)

		// case that new workable scheduler extender found
		sp = prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(workablePod),
			prepareValidatorClient(scheme, pendingPod, workablePod))
		sp.SelectedSchedulerExtenderIP = "192.168.1.2"
		sp.HTTPClient = server.Client()
		extenderIP, err = sp.getSchedulerExtenderIP(ctx, u.Port())
		assert.Nil(t, err)
		assert.Equal(t, extenderIP, u.Hostname())

		// workable selected scheduler extender case
		sp.SelectedSchedulerExtenderIP = u.Hostname()
		extenderIP, err = sp.getSchedulerExtenderIP(ctx, u.Port())
		assert.Nil(t, err)
		assert.Equal(t, extenderIP, u.Hostname())

	})
}

func Test_createOpenshiftConfig(t *testing.T) {
	t.Run("Test createOpenshiftConfig", func(t *testing.T) {
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()

		server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusOK)
			rw.Write([]byte(`OK`))
		}))
		defer server.Close()
		u, err := url.Parse(server.URL)
		if err != nil {
			t.Fatalf("Error in parsing server.URL %s", err.Error())
		}
		csiDeploy.Spec.Scheduler.ExtenderPort = u.Port()
		ctx := context.Background()

		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		sp.HTTPClient = server.Client()

		// error case
		config, err := sp.createOpenshiftConfig(ctx, csiDeploy, true)
		assert.NotNil(t, err)
		assert.Empty(t, config)

		// secondary scheduler config case
		sp.SelectedSchedulerExtenderIP = u.Hostname()
		config, err = sp.createOpenshiftConfig(ctx, csiDeploy, true)
		assert.Nil(t, err)
		assert.True(t, strings.HasPrefix(config, "apiVersion: kubescheduler.config.k8s.io/v1beta3"))

		// config case for secondary scheduler not used
		config, err = sp.createOpenshiftConfig(ctx, csiDeploy, false)
		assert.Nil(t, err)
		assert.False(t, strings.HasPrefix(config, "apiVersion: kubescheduler.config.k8s.io/v1beta3"))
	})
}

func Test_createOpenshiftConfigMapObject(t *testing.T) {
	t.Run("Test createOpenshiftConfigMapObject", func(t *testing.T) {
		expected := createOpenshiftConfigMapObject("data", true)
		assert.Equal(t, expected.Name, csiOpenshiftSecondarySchedulerConfigMapName)

		expected = createOpenshiftConfigMapObject("data", false)
		assert.Equal(t, expected.Name, openshiftSchedulerPolicyConfigMapName)
	})
}

func Test_SecondaryScheduler(t *testing.T) {
	t.Run("Test patch and unpatch SecondaryScheduler", func(t *testing.T) {
		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		ctx := context.Background()

		csiOpenshiftSecondarySchedulerDefaultImage := common.ConstructFullImageName(
			&components.Image{
				Name: openshiftSecondarySchedulerDefaultImageName,
				Tag:  openshiftSecondarySchedulerDefaultImageTag,
			}, csiDeploy.Spec.GlobalRegistry)
		// case that creates new SecondaryScheduler CR cluster
		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		secondarySchduler, err := sp.patchSecondaryScheduler(ctx, csiDeploy)
		assert.Equal(t, csiOpenshiftSecondarySchedulerConfigMapName, secondarySchduler.Spec.SchedulerConfig)
		assert.Equal(t, csiOpenshiftSecondarySchedulerDefaultImage, secondarySchduler.Spec.SchedulerImage)
		assert.Nil(t, err)

		// case of no update on existing SecondaryScheduler CR cluster
		csiDeploy.Spec.Scheduler.OpenshiftSecondaryScheduler = &components.OpenshiftSecondaryScheduler{
			Image: &components.Image{
				Name: "kube-scheduler",
			},
		}
		secondarySchduler, err = sp.patchSecondaryScheduler(ctx, csiDeploy)
		assert.Equal(t, csiOpenshiftSecondarySchedulerConfigMapName, secondarySchduler.Spec.SchedulerConfig)
		assert.Equal(t, csiOpenshiftSecondarySchedulerDefaultImage, secondarySchduler.Spec.SchedulerImage)
		assert.Nil(t, err)

		// case that update existing csi-baremetal secondary scheduler with different kube-scheduler image
		csiDeploy.Spec.Scheduler.OpenshiftSecondaryScheduler = &components.OpenshiftSecondaryScheduler{
			Image: &components.Image{
				Name: "kube-scheduler",
				Tag:  "v0.24.9",
			},
		}
		csiOpenshiftSecondarySchedulerImage := common.ConstructFullImageName(
			csiDeploy.Spec.Scheduler.OpenshiftSecondaryScheduler.Image, csiDeploy.Spec.GlobalRegistry)
		secondarySchduler, err = sp.patchSecondaryScheduler(ctx, csiDeploy)
		assert.Equal(t, csiOpenshiftSecondarySchedulerConfigMapName, secondarySchduler.Spec.SchedulerConfig)
		assert.Equal(t, csiOpenshiftSecondarySchedulerImage, secondarySchduler.Spec.SchedulerImage)
		assert.Nil(t, err)

		// uninstall secondaryscheduler
		err = sp.unpatchSecondaryScheduler(ctx)
		assert.Nil(t, err)
		err = sp.unpatchSecondaryScheduler(ctx)
		assert.NotNil(t, err)
		assert.True(t, k8sError.IsNotFound(err))

		// cases that try to update on existing 3rd-party SecondaryScheduler CR cluster
		secondarySchduler.Spec.SchedulerConfig = "config"
		sp = prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme, secondarySchduler))
		secondarySchduler, err = sp.patchSecondaryScheduler(ctx, csiDeploy)
		assert.Nil(t, secondarySchduler)
		assert.NotNil(t, err)
		assert.Equal(t, existing3rdPartySecondarySchedulerErrMsg, err.Error())

		err = sp.unpatchSecondaryScheduler(ctx)
		assert.Nil(t, err)
	})
}

func Test_PatchDisabled(t *testing.T) {
	t.Run("Test Kubernetes scheduler configuration patching not enabled", func(t *testing.T) {

		eventRecorder := new(mocks.EventRecorder)
		eventRecorder.On("Eventf", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
		scheme, _ := common.PrepareScheme()
		sp := prepareSchedulerPatcher(eventRecorder, prepareNodeClientSet(), prepareValidatorClient(scheme))
		csiDeploy.Spec.Scheduler.Patcher.Enable = false
		assert.Nil(t, sp.Update(context.Background(), csiDeploy, scheme))
	})
}
