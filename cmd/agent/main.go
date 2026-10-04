package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/pkg/application"
)

const (
	configPathEnv     = "METRIC_CONFIG_PATH"
	defaultConfigPath = "config/agent.yml"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("metrics-agent", flag.ContinueOnError)
	configPath := flags.String("config", "", "path to the configuration file")
	address := flags.String("a", "", "metrics server address")
	reportInterval := flags.Int("r", 0, "report interval in seconds")
	pollInterval := flags.Int("p", 0, "poll interval in seconds")
	if err := flags.Parse(args); err != nil {
		return errors.Wrap(err, "run agent. parse flags")
	}
	if flags.NArg() != 0 {
		return errors.Errorf("run agent. unexpected argument: %s", flags.Arg(0))
	}

	var overrides application.Overrides
	flags.Visit(func(current *flag.Flag) {
		switch current.Name {
		case "a":
			overrides.MetricsHTTPAddress = address
		case "r":
			interval := time.Duration(*reportInterval) * time.Second
			overrides.ReportInterval = &interval
		case "p":
			interval := time.Duration(*pollInterval) * time.Second
			overrides.PollInterval = &interval
		}
	})

	app, err := application.New(resolveConfigPath(*configPath), overrides)
	if err != nil {
		return errors.Wrap(err, "run agent. create application")
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
