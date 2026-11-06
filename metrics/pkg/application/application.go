package application

import (
	"context"
	stderrors "errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/config"
	pgmigrator "github.com/parta4ok/yandex-metrics/metrics/internal/adapter/migrator/postgres"
	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/osfile"
	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/storage/inmemory"
	pgstorage "github.com/parta4ok/yandex-metrics/metrics/internal/adapter/storage/postgres"
	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port/http/public"
	noopbackup "github.com/parta4ok/yandex-metrics/toolkit/backup/noop"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/zaplog"
	nooppinger "github.com/parta4ok/yandex-metrics/toolkit/pinger/noop"
)

type Application struct {
	ConfigProvider
	storage       cases.MetricsStorage
	backup        cases.BackupProvider
	pinger        cases.Pinger
	service       port.MetricServiceProvider
	publicServer  StartStopper
	startStoppers []StartStopper
	logger        toolkitlogger.Logger
}

func New(args []string, environ []string) (*Application, error) {
	config, err := config.NewConfig(args, environ)
	if err != nil {
		return nil, errors.Wrap(err, "new application. load config")
	}

	logger, err := zaplog.New()
	if err != nil {
		return nil, errors.Wrap(err, "new application. create logger")
	}

	return newApplication(config, logger)
}

func newApplication(config ConfigProvider, logger toolkitlogger.Logger) (*Application, error) {
	if config == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new application. config is nil")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new application. logger is nil")
	}

	return &Application{
		ConfigProvider: config,
		logger:         logger,
	}, nil
}

func (app *Application) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.build(ctx); err != nil {
		return app.handleError(err)
	}

	if err := app.restoreBackup(ctx); err != nil {
		return app.handleError(err)
	}

	if err := app.runStartStoppers(ctx); err != nil {
		return app.handleError(err)
	}

	return nil
}

func (app *Application) build(ctx context.Context) error {
	if err := app.buildStorage(ctx); err != nil {
		return errors.Wrap(err, "build application. storage")
	}
	if err := app.buildService(); err != nil {
		return errors.Wrap(err, "build application. metrics service")
	}
	if err := app.buildBackupMode(); err != nil {
		return errors.Wrap(err, "build application. backup mode")
	}
	if err := app.buildServer(); err != nil {
		return errors.Wrap(err, "build application. public HTTP server")
	}

	return nil
}

func (app *Application) buildStorage(ctx context.Context) error {
	if app.useDatabase() {
		return app.buildDatabaseStorage(ctx)
	}

	return app.buildMemoryStorage()
}

func (app *Application) buildMemoryStorage() error {
	backup, err := osfile.NewBackup(app.FileStoragePath(), app.logger)
	if err != nil {
		return errors.Wrap(err, "build memory storage. backup provider")
	}

	app.storage = inmemory.NewStorage(app.logger)
	app.pinger = nooppinger.New()
	app.backup = backup

	return nil
}

func (app *Application) buildDatabaseStorage(ctx context.Context) error {
	migrator, err := pgmigrator.NewMigrator(app.DatabaseDSN())
	if err != nil {
		return errors.Wrap(err, "build database storage. migrator")
	}

	storage, err := pgstorage.NewStorage(ctx, app.DatabaseDSN(), migrator)
	if err != nil {
		return errors.Wrap(err, "build database storage")
	}

	app.storage = storage
	app.pinger = storage
	app.backup = noopbackup.New[*entities.Metric]()
	app.startStoppers = append(app.startStoppers, storage)

	return nil
}

func (app *Application) useDatabase() bool {
	return app.DatabaseDSN() != ""
}

func (app *Application) buildService() error {
	service, err := cases.NewMetricsService(app.storage, app.backup, app.pinger, app.logger)
	if err != nil {
		return errors.Wrap(err, "build metrics service")
	}

	app.service = service

	return nil
}

func (app *Application) buildBackupMode() error {
	if app.useDatabase() {
		return nil
	}

	if app.StoreInterval() == 0 {
		service, err := cases.NewSyncBackupDecorator(app.service, app.logger)
		if err != nil {
			return errors.Wrap(err, "build sync backup decorator")
		}

		app.service = service

		return nil
	}

	ticker, err := NewBackupTicker(app.StoreInterval(), app.service, app.logger)
	if err != nil {
		return errors.Wrap(err, "build backup ticker")
	}

	app.startStoppers = append(app.startStoppers, ticker)

	return nil
}

func (app *Application) restoreBackup(ctx context.Context) error {
	if !app.Restore() {
		return nil
	}

	if err := app.service.RestoreBackup(ctx); err != nil {
		return errors.Wrap(err, "restore backup")
	}

	return nil
}

func (app *Application) buildServer() error {
	var options []public.Option
	if app.TLSEnabled() {
		options = append(
			options,
			public.WithTLS(
				app.TLSCertificateFile(),
				app.TLSKeyFile(),
			),
		)
	}

	server, err := public.NewServer(app.ConfigProvider, app.service, app.logger, options...)
	if err != nil {
		return errors.Wrap(err, "build public HTTP server")
	}

	app.publicServer = server
	app.startStoppers = append(app.startStoppers, app.publicServer)

	return nil
}

func (app *Application) runStartStoppers(ctx context.Context) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	componentErr := make(chan error, len(app.startStoppers))
	for _, startStopper := range app.startStoppers {
		go func(component StartStopper) {
			componentErr <- component.Start(ctx)
		}(startStopper)
	}

	select {
	case err := <-componentErr:
		stopErr := app.stopStartStoppers()
		if err != nil {
			if stopErr != nil {
				return errors.Wrapf(err, "run application. stop components failed: %v", stopErr)
			}

			return errors.Wrap(err, "run application. start component")
		}

		return stopErr
	case <-ctx.Done():
		return app.stopStartStoppers()
	}
}

func (app *Application) stopStartStoppers() error {
	ctx, cancel := context.WithTimeout(context.Background(), app.GracefulShutdownTimeout())
	defer cancel()

	componentErr := make(chan error, len(app.startStoppers))
	for _, startStopper := range app.startStoppers {
		go func(component StartStopper) {
			componentErr <- component.Stop(ctx)
		}(startStopper)
	}

	var componentErrs []error
	for range app.startStoppers {
		if err := <-componentErr; err != nil {
			componentErrs = append(componentErrs, errors.Wrap(err, "stop application component"))
		}
	}

	return stderrors.Join(componentErrs...)
}

func (app *Application) handleError(err error) error {
	return errors.WithStack(err)
}
