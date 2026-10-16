package main

import (
	"flag"
	"log"
	"os"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/pkg/application"
)

const (
	configPathEnv        = "METRIC_CONFIG_PATH"
	defaultConfigPath    = "config/agent.yml"
	defaultConfigExample = "config/agent.example.yml"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("metrics-agent", flag.ContinueOnError)
	configPath := flags.String("config", "", "path to the configuration file")
	flags.String("a", "", "metrics server address")
	flags.Int("r", 0, "report interval in seconds")
	flags.Int("p", 0, "poll interval in seconds")
	if err := flags.Parse(args); err != nil {
		return errors.Wrap(err, "run agent. parse flags")
	}
	if flags.NArg() != 0 {
		return errors.Errorf("run agent. unexpected argument: %s", flags.Arg(0))
	}

	app, err := application.New(
		resolveConfigPath(*configPath),
		application.WithFlags(flags),
		application.WithEnviron(os.Environ()),
	)
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

	if _, err := os.Stat(defaultConfigPath); os.IsNotExist(err) {
		return defaultConfigExample
	}

	return defaultConfigPath
}
