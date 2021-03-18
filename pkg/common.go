package pkg

const (
	CSIName = "csi-baremetal"
	// versions
	CSIVersion = "0.0.13-375.3c20841"

	// ports
	PrometheusPort = 8787
	LivenessPort   = "liveness-port"

	// timeouts
	TerminationGracePeriodSeconds = 10

	// volumes
	LogsVolume         = "logs"
	CSISocketDirVolume = "csi-socket-dir"

	// feature flags
	UseNodeAnnotation = false
)
