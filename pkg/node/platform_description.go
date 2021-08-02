package node

import (
	"github.com/dell/csi-baremetal-operator/api/v1/components"
	"github.com/masterminds/semver"
)

const supportedKernel = "5.4"

var supportedKernelVersion = semver.MustParse(supportedKernel)

type PlatformDescription struct {
	tag      string
	labeltag string
	checkVersion
}

type checkVersion func(version *semver.Version) bool

var (
	platforms = map[string]*PlatformDescription{
		"default": {
			tag:      "",
			labeltag: "default",
			// default checkVersion returns false everytime to detect only specific platforms
			checkVersion: func(version *semver.Version) bool { return false },
		},
		"kernel-5.4": {
			tag:          "kernel-5.4",
			labeltag:     "kernel-5.4",
			checkVersion: func(version *semver.Version) bool { return greaterOrEqual(version, supportedKernelVersion) },
		},
	}
)

func (pd *PlatformDescription) DaemonsetName(baseName string) string {
	return createNameWithTag(baseName, pd.tag)
}

func (pd *PlatformDescription) NodeImage(baseImage *components.Image) *components.Image {
	var taggedImage = components.Image{}

	taggedImage.Tag = baseImage.Tag
	taggedImage.Name = createNameWithTag(baseImage.Name, pd.tag)

	return &taggedImage
}

// findPlatform calls checkVersion for all platforms in list,
// returns first found platform-name or "default" if no one passed
func findPlatform(kernelVersion *semver.Version) string {
	for key, value := range platforms {
		if value.checkVersion(kernelVersion) {
			return key
		}
	}

	return "default"
}

func createNameWithTag(name, tag string) string {
	if tag != "" {
		return name + "-" + tag
	}

	return name
}

// greaterOrEqual returns true if version >= supported
func greaterOrEqual(version *semver.Version, supported *semver.Version) bool {
	return !version.LessThan(supported)
}
