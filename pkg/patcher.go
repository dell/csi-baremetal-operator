package pkg

import (
	"fmt"
	"path"

	csibaremetalv1 "github.com/dell/csi-baremetal-operator/api/v1"
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/dell/csi-baremetal-operator/pkg/common"
)

const (
	rke2ManifestsFolder    = "/var/lib/rancher/rke2/agent/pod-manifests"
	vanillaManifestsFolder = "/etc/kubernetes/manifests"

	schedulerFolder = "scheduler"

	policyFile   = "policy.yaml"
	configFile   = "config.yaml"
	config19File = "config-19.yaml"

	policyPath   = schedulerFolder + "/" + policyFile
	configPath   = schedulerFolder + "/" + configFile
	config19Path = schedulerFolder + "/" + config19File
)

func NewPatcherConfiguration(csi *csibaremetalv1.Deployment) (patcherConfiguration, error) {
	var config patcherConfiguration
	switch csi.Spec.Platform {
	case platformVanilla, "":
		config = patcherConfiguration{
			platform:        platformVanilla,
			targetConfig:    path.Join(vanillaManifestsFolder, configPath),
			targetPolicy:    path.Join(vanillaManifestsFolder, policyPath),
			targetConfig19:  path.Join(vanillaManifestsFolder, config19Path),
			schedulerFolder: path.Join(vanillaManifestsFolder, schedulerFolder),
			manifestsFolder: vanillaManifestsFolder,
		}
	case platformRKE:
		config = patcherConfiguration{
			platform:        platformRKE,
			targetConfig:    path.Join(rke2ManifestsFolder, configPath),
			targetPolicy:    path.Join(rke2ManifestsFolder, policyPath),
			targetConfig19:  path.Join(rke2ManifestsFolder, config19Path),
			schedulerFolder: path.Join(rke2ManifestsFolder, schedulerFolder),
			manifestsFolder: rke2ManifestsFolder,
		}
	default:
		return config, fmt.Errorf("%s platform is not supported platform for the patcher", csi.Spec.Platform)
	}
	config.enable = csi.Spec.Scheduler.Patcher.Enable
	config.image = csi.Spec.Scheduler.Patcher.Image
	config.interval = csi.Spec.Scheduler.Patcher.Interval
	config.restoreOnShutdown = csi.Spec.Scheduler.Patcher.RestoreOnShutdown
	config.configMapName = csi.Spec.Scheduler.Patcher.ConfigMapName
	config.ns = common.GetNamespace(csi)
	config.globalRegistry = csi.Spec.GlobalRegistry
	config.pullPolicy = csi.Spec.PullPolicy
	config.loglevel = csi.Spec.Scheduler.Log.Level
	config.configFolder = configurationPath
	config.schedulerFolder = schedulerFolder
	return config, nil
}

type patcherConfiguration struct {
	enable            bool
	ns                string
	image             *components.Image
	globalRegistry    string
	pullPolicy        string
	loglevel          components.Level
	interval          int
	restoreOnShutdown bool

	platform        string
	targetConfig    string
	targetPolicy    string
	targetConfig19  string
	schedulerFolder string
	manifestsFolder string
	configMapName   string
	configFolder    string
}
