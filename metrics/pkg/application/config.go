package application

import toolkitconfig "github.com/parta4ok/yandex-metrics/toolkit/config"

type PublicHTTPServerConfig interface {
	toolkitconfig.HTTPServerConfig
	toolkitconfig.TLSConfig
}

type ConfigProvider interface {
	toolkitconfig.GracefulStopConfig
	PublicHTTPServerConfig
}
