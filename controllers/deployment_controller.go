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
	"github.com/dell/csi-baremetal-operator/pkg"
	"github.com/dell/csi-baremetal-operator/pkg/scheduler"
	"k8s.io/client-go/kubernetes"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=csi-baremetal.dell.com,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=csi-baremetal.dell.com,resources=deployments/status,verbs=get;update;patch

func (r *DeploymentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("deployment", req.NamespacedName)

	deployment := new(csibaremetalv1.Deployment)
	err := r.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, deployment)
	if err != nil {
		log.Error(err, "Unable to read custom resource")
		return ctrl.Result{Requeue: true}, err
	}

	log.Info("Custom resource obtained")

	// todo CRDs?
	// todo SCs?
	// todo RBACs?
	config, _ := ctrl.GetConfig()
	k8sClient, _ := kubernetes.NewForConfig(config)
	// deploy node
	node := pkg.Node{Clientset: *k8sClient, Logger: r.Log.WithValues("node", req.NamespacedName)}
	if err = node.Create(req.Namespace); err != nil {
		log.Error(err, "Unable to deploy node service")
		return ctrl.Result{Requeue: true}, err
	}
	// deploy controller
	controller := pkg.Controller{Clientset: *k8sClient, Logger: r.Log.WithValues("controller", req.NamespacedName)}
	if err = controller.Create(req.Namespace); err != nil {
		log.Error(err, "Unable to deploy controller service")
		return ctrl.Result{Requeue: true}, err
	}
	// deploy scheduler extender
	extender := scheduler.Extender{Clientset: *k8sClient, Logger: r.Log.WithValues("extender", req.NamespacedName)}
	if err = extender.Create(req.Namespace); err != nil {
		log.Error(err, "Unable to deploy scheduler extender service")
		return ctrl.Result{Requeue: true}, err
	}
	// deploy scheduler patcher
	patcher := scheduler.Patcher{Clientset: *k8sClient, Logger: r.Log.WithValues("patcher", req.NamespacedName)}
	if err = patcher.Create(req.Namespace); err != nil {
		log.Error(err, "Unable to deploy scheduler patcher service")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&csibaremetalv1.Deployment{}).
		Complete(r)
}
