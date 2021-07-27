package patcher

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
)

func TestNewPatcherConfiguration(t *testing.T) {
	type args struct {
		csi *csibaremetalv1.Deployment
	}
	tests := []struct {
		name    string
		args    args
		want    *patcherConfiguration
		wantErr bool
	}{
		{
			name: "Vanilla kubernetes",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Interval:          10,
								RestoreOnShutdown: true,
								ConfigMapName:     "scheduler-configuration",
							},
						},
						NodeIDAnnotation: false,
						Platform:         "vanilla",
					},
				},
			},
			want: &patcherConfiguration{
				ns:                "default",
				loglevel:          "debug",
				interval:          10,
				restoreOnShutdown: true,
				platform:          "vanilla",
				targetConfig:      "/etc/kubernetes/manifests/scheduler/config.yaml",
				targetPolicy:      "/etc/kubernetes/manifests/scheduler/policy.yaml",
				targetConfig19:    "/etc/kubernetes/manifests/scheduler/config-19.yaml",
				schedulerFolder:   "/etc/kubernetes/manifests/scheduler",
				manifestsFolder:   "/etc/kubernetes/manifests",
				configMapName:     "scheduler-configuration",
				configFolder:      "/config",
				kubeconfig:        "/etc/kubernetes/scheduler.conf",
			},
			wantErr: false,
		},
		{
			name: "RKE kubernetes",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Interval:          10,
								RestoreOnShutdown: true,
								ConfigMapName:     "scheduler-conf",
							},
						},
						NodeIDAnnotation: false,
						Platform:         "rke",
					},
				},
			},
			want: &patcherConfiguration{
				ns:                "default",
				loglevel:          "debug",
				interval:          10,
				restoreOnShutdown: true,
				platform:          "rke",
				targetConfig:      "/var/lib/rancher/rke2/agent/pod-manifests/scheduler/config.yaml",
				targetPolicy:      "/var/lib/rancher/rke2/agent/pod-manifests/scheduler/policy.yaml",
				targetConfig19:    "/var/lib/rancher/rke2/agent/pod-manifests/scheduler/config-19.yaml",
				schedulerFolder:   "/var/lib/rancher/rke2/agent/pod-manifests/scheduler",
				manifestsFolder:   "/var/lib/rancher/rke2/agent/pod-manifests",
				configMapName:     "scheduler-conf",
				configFolder:      "/config",
				kubeconfig:        "/var/lib/rancher/rke2/server/cred/scheduler.kubeconfig",
			},
			wantErr: false,
		},
		{
			name: "Openshift kubernetes",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Interval:          10,
								RestoreOnShutdown: true,
								ConfigMapName:     "scheduler-conf",
							},
						},
						NodeIDAnnotation: false,
						Platform:         "openshift",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty platform",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Interval:          10,
								RestoreOnShutdown: true,
								ConfigMapName:     "scheduler-conf",
							},
						},
						NodeIDAnnotation: false,
						Platform:         "",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Typo in platform",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Interval:          10,
								RestoreOnShutdown: true,
								ConfigMapName:     "scheduler-conf",
							},
						},
						NodeIDAnnotation: false,
						Platform:         "vanila",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Unsupported platform",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "default",
					},
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Interval:          10,
								RestoreOnShutdown: true,
								ConfigMapName:     "scheduler-conf",
							},
						},
						NodeIDAnnotation: false,
						Platform:         "pks",
					},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newPatcherConfiguration(tt.args.csi)
			if (err != nil) != tt.wantErr {
				t.Errorf("newPatcherConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newPatcherConfiguration() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
