package application

import "time"

type Overrides struct {
	MetricsHTTPAddress *string
	PollInterval       *time.Duration
	ReportInterval     *time.Duration
}

type resolvedConfig struct {
	ConfigProvider
	metricsHTTPAddress string
	pollInterval       time.Duration
	reportInterval     time.Duration
}

func resolveConfig(config ConfigProvider, overrides Overrides) ConfigProvider {
	address := config.GetMetricsHTTPAddress()
	if overrides.MetricsHTTPAddress != nil {
		address = *overrides.MetricsHTTPAddress
	}

	pollInterval := config.GetPollInterval()
	if overrides.PollInterval != nil {
		pollInterval = *overrides.PollInterval
	}

	reportInterval := config.GetReportInterval()
	if overrides.ReportInterval != nil {
		reportInterval = *overrides.ReportInterval
	}

	return resolvedConfig{
		ConfigProvider:     config,
		metricsHTTPAddress: address,
		pollInterval:       pollInterval,
		reportInterval:     reportInterval,
	}
}

func (c resolvedConfig) GetMetricsHTTPAddress() string {
	return c.metricsHTTPAddress
}

func (c resolvedConfig) GetPollInterval() time.Duration {
	return c.pollInterval
}

func (c resolvedConfig) GetReportInterval() time.Duration {
	return c.reportInterval
}
