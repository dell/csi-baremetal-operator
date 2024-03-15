package patcher

import (
	"fmt"
	"path"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/constant"
)

const (
	rke2ManifestsFolder    = "/var/lib/rancher/rke2/agent/pod-manifests"
	vanillaManifestsFolder = "/etc/kubernetes/manifests"

	rke2Kubeconfig    = "/var/lib/rancher/rke2/server/cred/scheduler.kubeconfig"
	vanillaKubeconfig = "/etc/kubernetes/scheduler.conf"

	schedulerFolder = "scheduler"

	policyFile   = "policy.yaml"
	configFile   = "config.yaml"
	config19File = "config-19.yaml"
	config23File = "config-23.yaml"
	config29File = "config-29.yaml"

	policyPath   = schedulerFolder + "/" + policyFile
	configPath   = schedulerFolder + "/" + configFile
	config19Path = schedulerFolder + "/" + config19File
	config23Path = schedulerFolder + "/" + config23File
	config29Path = schedulerFolder + "/" + config29File
)

// newPatcherConfiguration creates patcherConfiguration
func newPatcherConfiguration(csi *csibaremetalv1.Deployment) (*patcherConfiguration, error) {
	var config patcherConfiguration
	switch csi.Spec.Platform {
	case constant.PlatformVanilla:
		config = patcherConfiguration{
			platform:        constant.PlatformVanilla,
			targetConfig:    path.Join(vanillaManifestsFolder, configPath),
			targetPolicy:    path.Join(vanillaManifestsFolder, policyPath),
			targetConfig19:  path.Join(vanillaManifestsFolder, config19Path),
			targetConfig23:  path.Join(vanillaManifestsFolder, config23Path),
			targetConfig29:  path.Join(vanillaManifestsFolder, config29Path),
			schedulerFolder: path.Join(vanillaManifestsFolder, schedulerFolder),
			manifestsFolder: vanillaManifestsFolder,
			kubeconfig:      vanillaKubeconfig,
		}
	case constant.PlatformRKE:
		config = patcherConfiguration{
			platform:        constant.PlatformRKE,
			targetConfig:    path.Join(rke2ManifestsFolder, configPath),
			targetPolicy:    path.Join(rke2ManifestsFolder, policyPath),
			targetConfig19:  path.Join(rke2ManifestsFolder, config19Path),
			targetConfig23:  path.Join(rke2ManifestsFolder, config23Path),
			targetConfig29:  path.Join(rke2ManifestsFolder, config29Path),
			schedulerFolder: path.Join(rke2ManifestsFolder, schedulerFolder),
			manifestsFolder: rke2ManifestsFolder,
			kubeconfig:      rke2Kubeconfig,
		}
	default:
		return nil, fmt.Errorf("%s platform is not supported platform for the patcher", csi.Spec.Platform)
	}
	config.image = csi.Spec.Scheduler.Patcher.Image
	config.interval = csi.Spec.Scheduler.Patcher.Interval
	config.restoreOnShutdown = csi.Spec.Scheduler.Patcher.RestoreOnShutdown
	config.configMapName = csi.Spec.Scheduler.Patcher.ConfigMapName
	config.ns = csi.GetNamespace()
	config.globalRegistry = csi.Spec.GlobalRegistry
	config.registrySecret = csi.Spec.RegistrySecret
	config.pullPolicy = csi.Spec.PullPolicy
	config.loglevel = csi.Spec.Scheduler.Log.Level
	config.configFolder = configurationPath
	config.resources = csi.Spec.Scheduler.Patcher.Resources
	config.securityContext = csi.Spec.Scheduler.SecurityContext
	config.serviceAccount = csi.Spec.Scheduler.ServiceAccount
	return &config, nil
}

type patcherConfiguration struct {
	ns                string
	image             *components.Image
	globalRegistry    string
	registrySecret    string
	pullPolicy        string
	loglevel          components.Level
	interval          int
	restoreOnShutdown bool
	resources         *components.ResourceRequirements
	securityContext   *components.SecurityContext

	platform        string
	targetConfig    string
	targetPolicy    string
	targetConfig19  string
	targetConfig23  string
	targetConfig29  string
	schedulerFolder string
	manifestsFolder string
	configMapName   string
	configFolder    string
	kubeconfig      string
	serviceAccount  string
}
