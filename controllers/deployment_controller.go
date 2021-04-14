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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	// TODO change log library - https://github.com/dell/csi-baremetal/issues/351
	"github.com/go-logr/logr"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg"
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

func (r *DeploymentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
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
			if err = r.UninstallPatcher(ctx, *deployment); err != nil {
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

	return ctrl.Result{}, nil
}

func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			// only one instance of CSIDeployment is allowed to be installed
			// concurrent reconciliation isn't supported
			MaxConcurrentReconciles: 1,
		}).
		Watches(&source.Kind{Type: &csibaremetalv1.Deployment{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &csibaremetalv1.Deployment{},
		}).
		Watches(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &csibaremetalv1.Deployment{},
		}).
		For(&csibaremetalv1.Deployment{}).
		Complete(r)
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
	var result []string
	for _, finalizer := range csiDep.ObjectMeta.Finalizers {
		if finalizer == csiFinalizer {
			continue
		}
		result = append(result, finalizer)
	}
	return result
}
