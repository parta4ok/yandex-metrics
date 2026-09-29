package application

import "time"

type ConfigProvider interface {
	GetGracefulShutdownTimeout() time.Duration
	GetPublicHTTPAddr() string
	IsPublicHTTPTLSEnabled() bool
	GetPublicHTTPTLSCertificateFile() string
	GetPublicHTTPTLSKeyFile() string
}
