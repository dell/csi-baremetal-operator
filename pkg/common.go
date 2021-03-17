package pkg

const (
	csiName = "csi-baremetal"
	// versions
	csiVersion = "0.0.13-375.3c20841"

	// ports
	prometheusPort = 8787
	livenessPort   = "liveness-port"

	// timeouts
	terminationGracePeriodSeconds = 10

	// volumes
	logsVolume         = "logs"
	csiSocketDirVolume = "csi-socket-dir"
)
