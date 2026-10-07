package application

import (
	"context"
	stderrors "errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/config"
	"github.com/parta4ok/yandex-metrics/metrics/internal/adapter/storage/inmemory"
	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port"
	"github.com/parta4ok/yandex-metrics/metrics/internal/port/http/public"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
	"github.com/parta4ok/yandex-metrics/toolkit/logger/baseslog"
)

type Application struct {
	ConfigProvider
	storage       cases.MetricsStorage
	service       port.MetricServiceProvider
	publicServer  StartStopper
	startStoppers []StartStopper
	logger        toolkitlogger.Logger
}

func New(configPath string, overrides Overrides) (*Application, error) {
	logger := baseslog.New()
	config, err := config.NewConfig(configPath, logger)
	if err != nil {
		return nil, errors.Wrap(err, "new application. load config")
	}

	return newApplication(resolveConfig(config, overrides), logger)
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
	if err := app.build(); err != nil {
		return app.handleError(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.runStartStoppers(ctx); err != nil {
		return app.handleError(err)
	}

	return nil
}

func (app *Application) build() error {
	app.buildStorage()

	if err := app.buildService(); err != nil {
		return errors.Wrap(err, "build application. metrics service")
	}
	if err := app.buildServer(); err != nil {
		return errors.Wrap(err, "build application. public HTTP server")
	}

	return nil
}

func (app *Application) buildStorage() {
	app.storage = inmemory.NewStorage(app.logger)
}

func (app *Application) buildService() error {
	service, err := cases.NewMetricsService(app.storage, app.logger)
	if err != nil {
		return errors.Wrap(err, "build metrics service")
	}

	app.service = service

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
