package application

import (
	"context"
	stderrors "errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/config"
	metricprovider "github.com/parta4ok/yandex-metrics/agent/internal/adapter/metric"
	metricsclient "github.com/parta4ok/yandex-metrics/agent/internal/adapter/metrics"
	"github.com/parta4ok/yandex-metrics/agent/internal/adapter/storage/inmemory"
	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

type Application struct {
	ConfigProvider
	storage             cases.Storage
	dataProvider        cases.DataProvider
	metricServiceClient cases.MetricServiceClient
	service             AgentServiceProvider
	startStoppers       []StartStopper
}

func New(configPath string) (*Application, error) {
	config, err := config.NewConfig(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "new application. load config")
	}

	return newApplication(config)
}

func newApplication(config ConfigProvider) (*Application, error) {
	if config == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new application. config is nil")
	}

	return &Application{
		ConfigProvider: config,
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
	app.buildDataProvider()

	if err := app.buildMetricServiceClient(); err != nil {
		return errors.Wrap(err, "build application. metrics service client")
	}
	if err := app.buildService(); err != nil {
		return errors.Wrap(err, "build application. agent service")
	}
	if err := app.buildTickers(); err != nil {
		return errors.Wrap(err, "build application. tickers")
	}

	return nil
}

func (app *Application) buildStorage() {
	app.storage = inmemory.NewStorage()
}

func (app *Application) buildDataProvider() {
	app.dataProvider = metricprovider.NewAgent()
}

func (app *Application) buildMetricServiceClient() error {
	client, err := metricsclient.NewClient(
		app.GetMetricsHTTPAddress(),
		&http.Client{
			Timeout: app.GetMetricsHTTPTimeout(),
		},
	)
	if err != nil {
		return errors.Wrap(err, "build metrics service client. create client")
	}

	app.metricServiceClient = client

	return nil
}

func (app *Application) buildService() error {
	service, err := cases.NewAgentService(
		app.metricServiceClient,
		app.dataProvider,
		app.storage,
	)
	if err != nil {
		return errors.Wrap(err, "build agent service. create service")
	}

	app.service = service

	return nil
}

func (app *Application) buildTickers() error {
	pollTicker, err := NewTicker(app.GetPollInterval(), app.service.UpdateMetrics)
	if err != nil {
		return errors.Wrap(err, "build tickers. create poll ticker")
	}

	reportTicker, err := NewTicker(app.GetReportInterval(), app.service.SendMetrics)
	if err != nil {
		return errors.Wrap(err, "build tickers. create report ticker")
	}

	app.startStoppers = append(app.startStoppers, pollTicker, reportTicker)

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
	ctx, cancel := context.WithTimeout(context.Background(), app.GetGracefulShutdownTimeout())
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
