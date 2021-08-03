/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"reflect"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	// TODO change log library - https://github.com/dell/csi-baremetal/issues/351
	"github.com/go-logr/logr"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg"
	"github.com/dell/csi-baremetal-operator/pkg/patcher"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	Client client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	pkg.CSIDeployment
}

const (
	csiFinalizer = "dell.emc.csi/csi-deployment-cleanup"
)

// +kubebuilder:rbac:groups=csi-baremetal.dell.com,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=csi-baremetal.dell.com,resources=deployments/status,verbs=get;update;patch

// Reconcile reconciles a Deployment object
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("deployment", req.NamespacedName)

	deployment := new(csibaremetalv1.Deployment)
	err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			// TODO set logLevel as Warn after changing log library
			// https://github.com/dell/csi-baremetal/issues/351
			log.Info("Custom resource is not found")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Unable to read custom resource")
		return ctrl.Result{Requeue: true}, err
	}
	log.Info("Custom resource obtained")

	if deployment.ObjectMeta.DeletionTimestamp.IsZero() {
		// Instance is not being deleted, add the finalizer if not present
		if !containsFinalizer(deployment) {
			deployment.ObjectMeta.Finalizers = append(deployment.ObjectMeta.Finalizers, csiFinalizer)
			if err = r.Client.Update(ctx, deployment); err != nil {
				log.Error(err, "Error adding finalizer")
				return ctrl.Result{Requeue: true}, err
			}

			log.Info("Successfully add finalizer to CR")
		}
	} else {
		if containsFinalizer(deployment) {
			if err = r.Uninstall(ctx, deployment); err != nil {
				log.Error(err, "Error uninstalling patcher")
			}

			deployment.ObjectMeta.Finalizers = deleteFinalizer(deployment)
			if err = r.Client.Update(ctx, deployment); err != nil {
				log.Error(err, "Error removing finalizer")
				return ctrl.Result{Requeue: true}, err
			}

			log.Info("Successfully remove finalizer")
			return ctrl.Result{}, nil
		}
	}

	if err = r.CSIDeployment.Update(ctx, deployment, r.Scheme); err != nil {
		log.Error(err, "Unable to update deployment")
		return ctrl.Result{Requeue: true}, err
	}

	if err = r.CSIDeployment.ReconcileNodes(ctx, deployment); err != nil {
		log.Error(err, "Failed to reconcile nodes")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager creates controller manager for CSI Deployment
func (r *DeploymentReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	c, err := controller.New("csi-controller", mgr,
		controller.Options{
			Reconciler: r,
			// only one instance of CSIDeployment is allowed to be installed
			// concurrent reconciliation isn't supported
			MaxConcurrentReconciles: 1,
		})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &csibaremetalv1.Deployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &csibaremetalv1.Deployment{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &csibaremetalv1.Deployment{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &csibaremetalv1.Deployment{},
	})
	if err != nil {
		return err
	}

	// reconcile CSI Deployment if kube-scheduler pods were changed
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		var (
			ctx         = context.Background()
			deployments = &csibaremetalv1.DeploymentList{}
			pod         *corev1.Pod
			ok          bool
		)

		err := r.Client.List(ctx, deployments)
		if err != nil {
			return []reconcile.Request{}
		}

		if pod, ok = obj.(*corev1.Pod); !ok {
			return []reconcile.Request{}
		}

		var requests []reconcile.Request
		for _, dep := range deployments.Items {
			depIns := dep

			// check kube-scheduler label
			// it depends on platform
			key, value, err := patcher.ChooseKubeSchedulerLabel(&depIns)
			if err != nil {
				continue
			}

			if realValue, ok := pod.GetLabels()[key]; !ok || value != realValue {
				continue
			}

			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      dep.Name,
					Namespace: dep.Namespace,
				}})
		}

		return requests
	}))
	if err != nil {
		return err
	}

	// reconcile CSI Deployment if node was creates, node kernel-version or label were changed
	err = c.Watch(&source.Kind{Type: &corev1.Node{}}, handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		var (
			ctx         = context.Background()
			deployments = &csibaremetalv1.DeploymentList{}
			node        *corev1.Node
			ok          bool
		)

		err := r.Client.List(ctx, deployments)
		if err != nil {
			return []reconcile.Request{}
		}

		if node, ok = obj.(*corev1.Node); !ok {
			return []reconcile.Request{}
		}

		var requests []reconcile.Request
		for _, dep := range deployments.Items {
			// check node has label from csi node selector
			// skip request if not
			if dep.Spec.NodeSelector != nil {
				value, ok := node.Labels[dep.Spec.NodeSelector.Key]
				if !ok || value != dep.Spec.NodeSelector.Value {
					continue
				}
			}

			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      dep.Name,
					Namespace: dep.Namespace,
				}})
		}

		return requests
	}), predicate.Or(predicate.Funcs{
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return isNodeChanged(updateEvent.ObjectOld, updateEvent.ObjectNew)
		},
	}))
	if err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1.Pod{}, "spec.nodeName", func(rawObj client.Object) []string {
		pod := rawObj.(*corev1.Pod)
		return []string{pod.Spec.NodeName}
	}); err != nil {
		return err
	}

	return nil
}

func isNodeChanged(old runtime.Object, new runtime.Object) bool {
	var (
		oldNode *corev1.Node
		newNode *corev1.Node
		ok      bool
	)
	if oldNode, ok = old.(*corev1.Node); !ok {
		return false
	}
	if newNode, ok = new.(*corev1.Node); !ok {
		return false
	}

	// labels
	if !reflect.DeepEqual(oldNode.Labels, newNode.Labels) {
		return true
	}

	// kernel version
	if oldNode.Status.NodeInfo.KernelVersion != newNode.Status.NodeInfo.KernelVersion {
		return true
	}

	// taints
	if !reflect.DeepEqual(oldNode.Spec.Taints, newNode.Spec.Taints) {
		return true
	}

	return false
}

func containsFinalizer(csiDep *csibaremetalv1.Deployment) bool {
	for _, finalizer := range csiDep.ObjectMeta.Finalizers {
		if strings.Contains(finalizer, csiFinalizer) {
			return true
		}
	}
	return false
}

func deleteFinalizer(csiDep *csibaremetalv1.Deployment) []string {
	result := make([]string, 0)
	for _, finalizer := range csiDep.ObjectMeta.Finalizers {
		if finalizer == csiFinalizer {
			continue
		}
		result = append(result, finalizer)
	}
	return result
}
