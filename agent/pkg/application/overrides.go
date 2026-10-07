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
	address := config.MetricsHTTPAddress()
	if overrides.MetricsHTTPAddress != nil {
		address = *overrides.MetricsHTTPAddress
	}

	pollInterval := config.PollInterval()
	if overrides.PollInterval != nil {
		pollInterval = *overrides.PollInterval
	}

	reportInterval := config.ReportInterval()
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

func (c resolvedConfig) MetricsHTTPAddress() string {
	return c.metricsHTTPAddress
}

func (c resolvedConfig) PollInterval() time.Duration {
	return c.pollInterval
}

func (c resolvedConfig) ReportInterval() time.Duration {
	return c.reportInterval
}
