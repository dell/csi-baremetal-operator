package node

import (
	"strconv"
)

const (
	supportedKernel = 5.4
)

type PlatformDescription struct {
	tag      string
	labeltag string
	checkVersion
}

type checkVersion func(version string) bool

var (
	platforms = map[string]*PlatformDescription{
		"default": {
			tag:      "",
			labeltag: "default",
			// default checkVersion returns false everytime to detect only specific platforms
			checkVersion: func(version string) bool { return false },
		},
		"kernel-5.4": {
			tag:          "kernel-5.4",
			labeltag:     "kernel-5.4",
			checkVersion: func(version string) bool { return moreThan(version, supportedKernel) },
		},
	}
)

func (pd *PlatformDescription) DaemonsetName(baseName string) string {
	return baseName + pd.tag
}

func (pd *PlatformDescription) ImageName(baseName string) string {
	return baseName + pd.tag
}

// moreThan returns true if version >= supported
func moreThan(version string, supported float64) bool {
	versionFloat, err := strconv.ParseFloat(version, 32)
	if err != nil {
		return false
	}

	if versionFloat >= supported {
		return true
	}

	return false
}
