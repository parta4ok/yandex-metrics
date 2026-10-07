package config

import "time"

type GracefulStopConfig interface {
	GracefulShutdownTimeout() time.Duration
}

type HTTPServerConfig interface {
	HTTPAddress() string
}

type TLSConfig interface {
	TLSEnabled() bool
	TLSCertificateFile() string
	TLSKeyFile() string
}
