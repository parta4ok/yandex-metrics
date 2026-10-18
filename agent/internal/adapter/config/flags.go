package config

import (
	"flag"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

const (
	configFlag         = "config"
	addressFlag        = "a"
	reportIntervalFlag = "r"
	pollIntervalFlag   = "p"
)

type cliFlags struct {
	set            *flag.FlagSet
	explicit       map[string]struct{}
	configPath     string
	address        string
	reportInterval int
	pollInterval   int
}

func newCLIFlags() *cliFlags {
	flags := &cliFlags{
		set:      flag.NewFlagSet("metrics-agent", flag.ContinueOnError),
		explicit: make(map[string]struct{}),
	}

	flags.set.StringVar(&flags.configPath, configFlag, "", "path to the configuration file")
	flags.set.StringVar(&flags.address, addressFlag, "", "metrics server address (default localhost:8080)")
	flags.set.IntVar(&flags.reportInterval, reportIntervalFlag, 0, "report interval in seconds (default 10)")
	flags.set.IntVar(&flags.pollInterval, pollIntervalFlag, 0, "poll interval in seconds (default 2)")

	return flags
}

func (f *cliFlags) parse(args []string) error {
	if err := f.set.Parse(args); err != nil {
		return errors.Wrapf(entities.ErrInvalidParam, "%v", err)
	}
	if f.set.NArg() != 0 {
		return errors.Wrapf(entities.ErrInvalidParam, "unexpected argument: %s", f.set.Arg(0))
	}

	f.set.Visit(func(flag *flag.Flag) {
		f.explicit[flag.Name] = struct{}{}
	})

	return nil
}

func (f *cliFlags) isSet(name string) bool {
	_, ok := f.explicit[name]

	return ok
}
