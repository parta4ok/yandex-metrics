package application

type Overrides struct {
	PublicHTTPAddress *string
}

type resolvedConfig struct {
	ConfigProvider
	publicHTTPAddress string
}

func resolveConfig(config ConfigProvider, overrides Overrides) ConfigProvider {
	address := config.GetPublicHTTPAddr()
	if overrides.PublicHTTPAddress != nil {
		address = *overrides.PublicHTTPAddress
	}

	return resolvedConfig{
		ConfigProvider:    config,
		publicHTTPAddress: address,
	}
}

func (c resolvedConfig) GetPublicHTTPAddr() string {
	return c.publicHTTPAddress
}
