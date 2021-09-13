package patcher

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	schedulerConf = "scheduler-conf"
	ns            = "default"
)

func Test_NewExtenderReadinessOptions(t *testing.T) {
	type args struct {
		csi *csibaremetalv1.Deployment
	}

	tests := []struct {
		name    string
		args    args
		want    *ExtenderReadinessOptions
		wantErr bool
	}{
		{
			name: "Openshift",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
					},
					Spec: components.DeploymentSpec{
						Platform: "openshift",
						Scheduler: &components.Scheduler{
							Patcher: &components.Patcher{
								ConfigMapName: schedulerConf,
							},
						},
					},
				},
			},
			want: &ExtenderReadinessOptions{
				watchedConfigMapName:        "scheduler-policy",
				watchedConfigMapNamespace:   "openshift-config",
				readinessConfigMapName:      "extender-readiness",
				readinessConfigMapNamespace: ns,
				readinessConfigMapFile:      "nodes.yaml",
				kubeSchedulerLabel:          "app=openshift-kube-scheduler",
			},
			wantErr: false,
		},
		{
			name: "Vanilla",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
					},
					Spec: components.DeploymentSpec{
						Platform: "vanilla",
						Scheduler: &components.Scheduler{
							Patcher: &components.Patcher{
								ConfigMapName: schedulerConf,
							},
						},
					},
				},
			},
			want: &ExtenderReadinessOptions{
				watchedConfigMapName:        schedulerConf,
				watchedConfigMapNamespace:   ns,
				readinessConfigMapName:      "extender-readiness",
				readinessConfigMapNamespace: ns,
				readinessConfigMapFile:      "nodes.yaml",
				kubeSchedulerLabel:          "component=kube-scheduler",
			},
			wantErr: false,
		},
		{
			name: "RKE",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
					},
					Spec: components.DeploymentSpec{
						Platform: "rke",
						Scheduler: &components.Scheduler{
							Patcher: &components.Patcher{
								ConfigMapName: schedulerConf,
							},
						},
					},
				},
			},
			want: &ExtenderReadinessOptions{
				watchedConfigMapName:        schedulerConf,
				watchedConfigMapNamespace:   ns,
				readinessConfigMapName:      "extender-readiness",
				readinessConfigMapNamespace: ns,
				readinessConfigMapFile:      "nodes.yaml",
				kubeSchedulerLabel:          "component=kube-scheduler",
			},
			wantErr: false,
		},
		{
			name: "Unsupported",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
					},
					Spec: components.DeploymentSpec{
						Platform: "unsupported",
						Scheduler: &components.Scheduler{
							Patcher: &components.Patcher{
								ConfigMapName: schedulerConf,
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: ns,
					},
					Spec: components.DeploymentSpec{
						Platform: "",
						Scheduler: &components.Scheduler{
							Patcher: &components.Patcher{
								ConfigMapName: schedulerConf,
							},
						},
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExtenderReadinessOptions(tt.args.csi)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExtenderReadinessOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewExtenderReadinessOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_createReadinessConfigMap(t *testing.T) {
	var (
		csi = &csibaremetalv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: ns,
			},
			Spec: components.DeploymentSpec{
				Platform: "vanilla",
				Scheduler: &components.Scheduler{
					Patcher: &components.Patcher{
						ConfigMapName: schedulerConf,
					},
				},
			},
		}
	)
	t.Run("Success", func(t *testing.T) {
		var (
			nodeName      = "node"
			schedulerName = "scheduler"
			statuses      = &ReadinessStatusList{
				Items: []ReadinessStatus{
					{NodeName: nodeName, KubeScheduler: schedulerName, Restarted: true},
				}}
			savedStatuses = &ReadinessStatusList{}
		)

		options, err := NewExtenderReadinessOptions(csi)
		assert.Nil(t, err)

		config, err := createReadinessConfigMap(options, statuses)
		assert.Nil(t, err)
		assert.Equal(t, options.readinessConfigMapNamespace, config.GetNamespace())
		assert.Equal(t, options.readinessConfigMapName, config.Name)

		err = yaml.Unmarshal([]byte(config.Data[options.readinessConfigMapFile]), savedStatuses)
		assert.Nil(t, err)
		assert.Equal(t, statuses, savedStatuses)
	})
}

func Test_updateReadinessStatuses(t *testing.T) {
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
	t.Run("Success", func(t *testing.T) {
		var (
			pod1 = podTemplate.DeepCopy()
			pod2 = podTemplate.DeepCopy()
			pod3 = podTemplate.DeepCopy()
		)
		pod1.Name = pod1.Name + "1"
		pod1.Spec.NodeName = pod1.Spec.NodeName + "1"
		pod1.Status.ContainerStatuses[0].State.Running.StartedAt.Time = curTime.Add(time.Minute)

		pod2.Name = pod2.Name + "2"
		pod2.Spec.NodeName = pod2.Spec.NodeName + "1"
		pod2.Status.ContainerStatuses[0].State.Running.StartedAt.Time = curTime.Add(-time.Minute)

		pod3.Name = pod3.Name + "3"
		pod3.Spec.NodeName = pod3.Spec.NodeName + "1"
		pod3.Status.ContainerStatuses[0].State.Running.StartedAt.Time = curTime

		sp := prepareSchedulerPatcher(pod1, pod2, pod3)

		statuses, err := sp.updateReadinessStatuses(ctx, "component=kube-scheduler", metav1.Time{Time: curTime})
		assert.Nil(t, err)
		assert.Equal(t, 3, len(statuses.Items))

		for _, status := range statuses.Items {
			if status.KubeScheduler == pod1.Name {
				assert.Equal(t, pod1.Spec.NodeName, status.NodeName)
				assert.True(t, status.Restarted)
			}
			if status.KubeScheduler == pod2.Name {
				assert.Equal(t, pod2.Spec.NodeName, status.NodeName)
				assert.False(t, status.Restarted)
			}
			if status.KubeScheduler == pod3.Name {
				assert.Equal(t, pod3.Spec.NodeName, status.NodeName)
				assert.True(t, status.Restarted)
			}
		}
	})
}

func Test_IsPatchingEnabled(t *testing.T) {
	csi := &csibaremetalv1.Deployment{
		Spec: components.DeploymentSpec{
			Platform: constant.PlatformVanilla,
			Scheduler: &components.Scheduler{
				Patcher: &components.Patcher{
					Enable: true,
				},
			},
		},
	}
	// check vanilla
	result := IsPatchingEnabled(csi)
	assert.True(t, result)
	// check OpenShift
	csi.Spec.Platform = constant.PlatformOpenShift
	result = IsPatchingEnabled(csi)
	assert.True(t, result)
	// check RKE
	csi.Spec.Platform = constant.PlatformRKE
	result = IsPatchingEnabled(csi)
	assert.True(t, result)
	// check not supported
	csi.Spec.Platform = "other"
	result = IsPatchingEnabled(csi)
	assert.False(t, result)
	// check not set
	csi.Spec.Platform = constant.PlatformVanilla
	csi.Spec.Scheduler.Patcher.Enable = false
	result = IsPatchingEnabled(csi)
	assert.False(t, result)
}

func prepareSchedulerPatcher(objects ...runtime.Object) *SchedulerPatcher {
	clientset := fake.NewSimpleClientset(objects...)
	sp := &SchedulerPatcher{
		clientset,
		ctrl.Log.WithName("SchedulerPatcherTest"),
		nil,
	}

	return sp
}
