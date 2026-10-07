package application

import (
	"time"

	toolkitconfig "github.com/parta4ok/yandex-metrics/toolkit/config"
)

type MetricsHTTPClientConfig interface {
	MetricsHTTPAddress() string
	MetricsHTTPTimeout() time.Duration
}

type TickerConfig interface {
	PollInterval() time.Duration
	ReportInterval() time.Duration
}

type ConfigProvider interface {
	toolkitconfig.GracefulStopConfig
	MetricsHTTPClientConfig
	TickerConfig
}
