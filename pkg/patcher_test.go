package pkg

import (
	"reflect"
	"testing"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewPatcherConfiguration(t *testing.T) {
	type args struct {
		csi *csibaremetalv1.Deployment
	}
	tests := []struct {
		name    string
		args    args
		want    patcherConfiguration
		wantErr bool
	}{
		{
			name: "Vanilla kubernetes",
			args: args{
				csi: &csibaremetalv1.Deployment{
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Enable:            true,
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
			want: patcherConfiguration{
				enable:            true,
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
			},
			wantErr: false,
		},
		{
			name: "Vanilla kubernetes with empty platform field",
			args: args{
				csi: &csibaremetalv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Namespace: "csi"},
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Enable:            true,
								Interval:          10,
								RestoreOnShutdown: true,
								ConfigMapName:     "scheduler-configuration",
							},
						},
						NodeIDAnnotation: false,
						Platform:         "",
					},
				},
			},
			want: patcherConfiguration{
				enable:            true,
				ns:                "csi",
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
			},
			wantErr: false,
		},
		{
			name: "RKE kubernetes",
			args: args{
				csi: &csibaremetalv1.Deployment{
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Enable:            true,
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
			want: patcherConfiguration{
				enable:            true,
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
			},
			wantErr: false,
		},
		{
			name: "Openshift kubernetes",
			args: args{
				csi: &csibaremetalv1.Deployment{
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Enable:            true,
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
			want:    patcherConfiguration{},
			wantErr: true,
		},
		{
			name: "Typo in platform",
			args: args{
				csi: &csibaremetalv1.Deployment{
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Enable:            true,
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
			want:    patcherConfiguration{},
			wantErr: true,
		},
		{
			name: "Unsupported platform",
			args: args{
				csi: &csibaremetalv1.Deployment{
					Spec: components.DeploymentSpec{
						Scheduler: &components.Scheduler{
							Log: &components.Log{
								Level: "debug",
							},
							Patcher: &components.Patcher{
								Enable:            true,
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
			want:    patcherConfiguration{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewPatcherConfiguration(tt.args.csi)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPatcherConfiguration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewPatcherConfiguration() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
