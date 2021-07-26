package patcher

import (
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
)

const (
	schedulerConf = "scheduler-conf"
	ns            = "default"
)

func TestNewExtenderReadinessOptions(t *testing.T) {
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

func TestCreateReadinessConfigMap(t *testing.T) {
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

/*func prepareNode(objects ...runtime.Object) *Node{
	clientset := fake.NewSimpleClientset(objects...)
	node := NewNode(clientset, ctrl.Log.WithName("NodeTest"))
}*/
