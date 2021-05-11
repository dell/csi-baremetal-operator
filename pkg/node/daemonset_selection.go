package node

import (
	"strconv"
)

type PlatformDescription struct {
	imageName     string
	daemonsetName string
	labeltag      string
	checkVersion
}

type checkVersion func(version string) bool

var (
	platforms = map[string]*PlatformDescription{
		"default": {
			imageName:     "csi-baremetal-node",
			daemonsetName: "csi-baremetal-node",
			labeltag:      "default",
			checkVersion:  func(version string) bool { return false },
		},
		"kernel-5.4": {
			imageName:     "csi-baremetal-node-kernel-5.4",
			daemonsetName: "csi-baremetal-node-kernel-5.4",
			labeltag:      "kernel-5.4",
			checkVersion:  moreThan,
		},
	}
)

func moreThan(version string) bool {
	number := 5.4
	versionFloat, err := strconv.ParseFloat(version, 32)
	if err != nil {
		return false
	}

	if versionFloat >= number {
		return true
	}

	return false
}
