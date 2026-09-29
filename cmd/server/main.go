package main

import (
	"flag"
	"log"
	"os"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/pkg/application"
)

const (
	configPathEnv     = "METRIC_CONFIG_PATH"
	defaultConfigPath = "config/metrics.yml"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to the configuration file")
	flag.Parse()

	app, err := application.New(resolveConfigPath(*configPath))
	if err != nil {
		return errors.Wrap(err, "run metrics service. create application")
	}

	return app.Run()
}

func resolveConfigPath(configPath string) string {
	if configPath != "" {
		return configPath
	}

	if configPath := os.Getenv(configPathEnv); configPath != "" {
		return configPath
	}

	return defaultConfigPath
}
