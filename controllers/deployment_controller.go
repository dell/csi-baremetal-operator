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

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/pkg"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/patcher"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
)

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	Client client.Client
	Log    *logrus.Entry
	Scheme *runtime.Scheme
	pkg.CSIDeployment
	Matcher                                 rbac.Matcher
	MatchPodSecurityPolicyTemplate          rbacv1.PolicyRule
	MatchSecurityContextConstraintsPolicies []rbacv1.PolicyRule
}

const (
	csiFinalizer = "dell.emc.csi/csi-deployment-cleanup"
)

// +kubebuilder:rbac:groups=csi-baremetal.dell.com,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=csi-baremetal.dell.com,resources=deployments/status,verbs=get;update;patch

// Reconcile reconciles a Deployment object
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithField("deployment", req.NamespacedName)

	deployment := new(csibaremetalv1.Deployment)
	err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, deployment)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Warn("Custom resource is not found")
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

	// reconcile CSI Deployment if kube-scheduler or openshift secondary-scheduler pods were changed
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

			// check kube-scheduler label or openshift secondary-scheduler label
			// it depends on platform
			key, value, err := patcher.ChooseKubeSchedulerLabel(&depIns)
			if err != nil {
				continue
			}

			if realValue, ok := pod.GetLabels()[key]; !ok ||
				(value != realValue && value != patcher.OpenshiftSecondarySchedulerLabelValue) {
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

	if err = watchRole(c, r.Client, r.Matcher, r.MatchPodSecurityPolicyTemplate, r.MatchSecurityContextConstraintsPolicies, r.Log); err != nil {
		return err
	}

	if err = watchRoleBinding(c, r.Client, r.Matcher, r.Log); err != nil {
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

func watchRole(c controller.Controller, cl client.Client, m rbac.Matcher,
	matchPodSecurityPolicyTemplate rbacv1.PolicyRule, matchSecurityContextConstraintsPolicies []rbacv1.PolicyRule,
	log *logrus.Entry,
) error {
	return c.Watch(&source.Kind{Type: &rbacv1.Role{}}, handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		var (
			ctx         = context.Background()
			deployments = &csibaremetalv1.DeploymentList{}
			role        = &rbacv1.Role{}
			ok          bool
		)

		err := cl.List(ctx, deployments)
		if err != nil {
			log.Error(err, "Failed to list csi deployments")
			return []reconcile.Request{}
		}

		if len(deployments.Items) == 0 {
			// if there are no deployments - then just skip reconcile
			return []reconcile.Request{}
		}

		if role, ok = obj.(*rbacv1.Role); !ok {
			log.Warnf("got invalid Object type at Role watcher, actual type: '%s'", reflect.TypeOf(obj))
			return []reconcile.Request{}
		}

		if len(deployments.Items) != 1 {
			log.Warnf("Invalid number of csi deployments at Role watcher, number: '%d', expected: '%d'",
				len(deployments.Items), 1)
			return []reconcile.Request{}
		}

		// Reconcile roles for openshift platform and non default namespace
		securityContextConstraintsCondition := deployments.Items[0].Spec.Platform == constant.PlatformOpenShift &&
			deployments.Items[0].Namespace != constant.DefaultNamespace &&
			deployments.Items[0].Namespace == role.Namespace && m.MatchPolicyRules(role.Rules, matchSecurityContextConstraintsPolicies)
		// Reconcile roles if pod security policy is enabled for node
		var podNodeSecurityPolicyCondition bool
		if deployments.Items[0].Spec.Driver.Node.PodSecurityPolicy != nil && deployments.Items[0].Spec.Driver.Node.PodSecurityPolicy.Enable {
			matchPodSecurityPolicyTemplate.ResourceNames = []string{deployments.Items[0].Spec.Driver.Node.PodSecurityPolicy.ResourceName}
			podNodeSecurityPolicyCondition = m.MatchPolicyRules(role.Rules, matchSecurityContextConstraintsPolicies)
		}
		// Reconcile roles if pod security policy is enabled for scheduler
		var podSchedulerSecurityPolicyCondition bool
		if deployments.Items[0].Spec.Scheduler.PodSecurityPolicy != nil && deployments.Items[0].Spec.Scheduler.PodSecurityPolicy.Enable {
			matchPodSecurityPolicyTemplate.ResourceNames = []string{deployments.Items[0].Spec.Scheduler.PodSecurityPolicy.ResourceName}
			podSchedulerSecurityPolicyCondition = m.MatchPolicyRules(role.Rules, matchSecurityContextConstraintsPolicies)
		}
		if !securityContextConstraintsCondition && !podNodeSecurityPolicyCondition && !podSchedulerSecurityPolicyCondition {
			return []reconcile.Request{}
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      deployments.Items[0].Name,
					Namespace: deployments.Items[0].Namespace,
				},
			},
		}
	}))
}

func watchRoleBinding(c controller.Controller, cl client.Client, m rbac.Matcher, log *logrus.Entry) error {
	return c.Watch(&source.Kind{Type: &rbacv1.RoleBinding{}}, handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		var (
			ctx         = context.Background()
			deployments = &csibaremetalv1.DeploymentList{}
			roleBinding = &rbacv1.RoleBinding{}
			ok          bool
		)

		err := cl.List(ctx, deployments)
		if err != nil {
			log.Error(err, "Failed to list csi deployments")
			return []reconcile.Request{}
		}

		if len(deployments.Items) == 0 {
			// if there are no deployments - then just skip reconcile
			return []reconcile.Request{}
		}

		if roleBinding, ok = obj.(*rbacv1.RoleBinding); !ok {
			log.Warnf("got invalid Object type at RoleBinding watcher, actual type: '%s'", reflect.TypeOf(obj))
			return []reconcile.Request{}
		}

		if len(deployments.Items) != 1 {
			log.Warnf("Invalid number of csi deployments at RoleBinding watcher, number: '%d', expected: '%d'",
				len(deployments.Items), 1)
			return []reconcile.Request{}
		}

		// Checking, whether rolebinding matching the passed serviceAccounts
		matchNodeRoleBindingSubject := m.MatchRoleBindingSubjects(roleBinding, deployments.Items[0].Spec.Driver.Node.ServiceAccount, deployments.Items[0].Namespace)
		matchSchedulerRoleBindingSubject := m.MatchRoleBindingSubjects(roleBinding, deployments.Items[0].Spec.Scheduler.ServiceAccount, deployments.Items[0].Namespace)
		// Reconcile rolebindings for openshift platform and non default namespace and only on node and scheduler extender service accounts
		securityContextConstraintsCondition := deployments.Items[0].Spec.Platform == constant.PlatformOpenShift &&
			deployments.Items[0].Namespace != constant.DefaultNamespace &&
			deployments.Items[0].Namespace == roleBinding.Namespace && (matchNodeRoleBindingSubject || matchSchedulerRoleBindingSubject)
		// Reconcile rolebindings if pod security policy is enabled for node
		podNodeSecurityPolicyCondition := deployments.Items[0].Spec.Driver.Node.PodSecurityPolicy != nil &&
			deployments.Items[0].Spec.Driver.Node.PodSecurityPolicy.Enable && matchNodeRoleBindingSubject
		// Reconcile rolebindings if pod security policy is enabled for scheduler extender
		podSchedulerSecurityPolicyCondition := deployments.Items[0].Spec.Scheduler.PodSecurityPolicy != nil &&
			deployments.Items[0].Spec.Scheduler.PodSecurityPolicy.Enable && matchSchedulerRoleBindingSubject
		if !securityContextConstraintsCondition && !podNodeSecurityPolicyCondition && !podSchedulerSecurityPolicyCondition {
			return []reconcile.Request{}
		}

		return []reconcile.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      deployments.Items[0].Name,
					Namespace: deployments.Items[0].Namespace,
				},
			},
		}
	}))
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
