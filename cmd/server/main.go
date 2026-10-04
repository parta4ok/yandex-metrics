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
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("metrics-server", flag.ContinueOnError)
	configPath := flags.String("config", "", "path to the configuration file")
	address := flags.String("a", "", "HTTP server address")
	if err := flags.Parse(args); err != nil {
		return errors.Wrap(err, "run metrics service. parse flags")
	}
	if flags.NArg() != 0 {
		return errors.Errorf("run metrics service. unexpected argument: %s", flags.Arg(0))
	}

	var overrides application.Overrides
	flags.Visit(func(current *flag.Flag) {
		if current.Name == "a" {
			overrides.PublicHTTPAddress = address
		}
	})

	app, err := application.New(resolveConfigPath(*configPath), overrides)
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
