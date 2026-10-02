package application

import "time"

type ConfigProvider interface {
	GetGracefulShutdownTimeout() time.Duration
	GetPollInterval() time.Duration
	GetReportInterval() time.Duration
	GetMetricsHTTPAddress() string
	GetMetricsHTTPTimeout() time.Duration
}
