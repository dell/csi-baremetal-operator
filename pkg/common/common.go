package common

import (
	"github.com/dell/csi-baremetal-operator/api/v1/components"
)

// MatchLogLevel checks if passed logLevel is allowed
func MatchLogLevel(level components.Level) string {
	switch level {
	case components.InfoLevel:
		return string(level)
	case components.DebugLevel:
		return string(level)
	case components.TraceLevel:
		return string(level)

	default:
		return string(components.InfoLevel)
	}
}

// MatchLogFormat checks if passed logFormat is allowed
func MatchLogFormat(format components.Format) string {
	switch format {
	case components.JSONFormat:
		return string(format)
	case components.TextFormat:
		return string(format)

	default:
		return string(components.TextFormat)
	}
}

// ConstructFullImageName returns name of image in the following format: <registry>/<image_name>:<image_tag>
func ConstructFullImageName(image *components.Image, registry string) string {
	var imageName string

	if registry != "" {
		imageName += registry + "/"
	}

	imageName += image.Name + ":" + image.Tag
	return imageName
}

// MakeNodeSelectorMap creates map with node selector from csi spec
func MakeNodeSelectorMap(ns *components.NodeSelector) map[string]string {
	if ns != nil {
		return map[string]string{ns.Key: ns.Value}
	}

	return map[string]string{}
}
