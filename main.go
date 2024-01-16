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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/dell/csi-baremetal/pkg/events/recorder"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/dell/csi-baremetal-operator/controllers"
	"github.com/dell/csi-baremetal-operator/pkg"
	"github.com/dell/csi-baremetal-operator/pkg/acrvalidator"
	"github.com/dell/csi-baremetal-operator/pkg/common"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac"
	// +kubebuilder:scaffold:imports
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func InitLogger(logLevel string) *logrus.Logger {
	logger := logrus.New()

	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}

	logger.SetLevel(level)
	return logger
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var logLevel string
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&logLevel, "loglevel", "info", fmt.Sprintf("Log level, support values are %s, %s, %s, %s, %s, %s, %s",
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
		logrus.TraceLevel))
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	config := ctrl.GetConfigOrDie()
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to setup client set")
		os.Exit(1)
	}

	scheme, err := common.PrepareScheme()
	if err != nil {
		setupLog.Error(err, "unable to setup scheme")
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "7db7c6a0.dell.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	acrvalidator.LauncACRValidation(mgr.GetClient(), logrus.WithField("component", "acr_validator"))

	ctx := context.Background()
	logger := InitLogger(logLevel)

	eventRecorder := recorder.New(&v1core.EventSinkImpl{Interface: clientSet.CoreV1().Events("")},
		scheme, corev1.EventSource{Component: constant.ComponentName},
		logger.WithField(constant.CSIName, "eventRecorder"),
	)
	if err != nil {
		setupLog.Error(err, "unable to setup event recorder")
		os.Exit(1)
	}
	matcher := rbac.NewMatcher()
	matchSecurityContextConstraintsPolicies := []rbacv1.PolicyRule{
		{
			Verbs:         []string{"use"},
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			ResourceNames: []string{"privileged"},
		},
	}
	matchPodSecurityPolicyTemplate := rbacv1.PolicyRule{
		Verbs:     []string{"use"},
		APIGroups: []string{"policy"},
		Resources: []string{"podsecuritypolicies"},
	}
	if err = (&controllers.DeploymentReconciler{
		Client: mgr.GetClient(),
		Log: logrus.WithFields(logrus.Fields{
			"module": "controllers", "component": "DeploymentReconciler"}),
		Scheme: mgr.GetScheme(),
		CSIDeployment: pkg.NewCSIDeployment(*clientSet, mgr.GetClient(),
			matcher, matchSecurityContextConstraintsPolicies, matchPodSecurityPolicyTemplate,
			eventRecorder, logger,
		),
		Matcher:                                 matcher,
		MatchPodSecurityPolicyTemplate:          matchPodSecurityPolicyTemplate,
		MatchSecurityContextConstraintsPolicies: matchSecurityContextConstraintsPolicies,
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Deployment")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
