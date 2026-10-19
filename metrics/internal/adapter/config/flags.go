package config

import (
	"flag"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
)

const (
	configFlag          = "config"
	addressFlag         = "a"
	storeIntervalFlag   = "i"
	fileStoragePathFlag = "f"
	restoreFlag         = "r"
)

type cliFlags struct {
	set             *flag.FlagSet
	explicit        map[string]struct{}
	configPath      string
	address         string
	storeInterval   int
	fileStoragePath string
	restore         bool
}

func newCLIFlags() *cliFlags {
	flags := &cliFlags{
		set:      flag.NewFlagSet("metrics-server", flag.ContinueOnError),
		explicit: make(map[string]struct{}),
	}

	flags.set.StringVar(&flags.configPath, configFlag, "", "path to the configuration file")
	flags.set.StringVar(&flags.address, addressFlag, "", "HTTP server address (default localhost:8080)")
	flags.set.IntVar(
		&flags.storeInterval,
		storeIntervalFlag,
		0,
		"store interval in seconds, 0 makes backup synchronous (default 300)",
	)
	flags.set.StringVar(
		&flags.fileStoragePath,
		fileStoragePathFlag,
		"",
		"path to the metrics backup file (default metrics-storage.json)",
	)
	flags.set.BoolVar(
		&flags.restore,
		restoreFlag,
		false,
		"restore metrics from the backup file on start (default false)",
	)

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
