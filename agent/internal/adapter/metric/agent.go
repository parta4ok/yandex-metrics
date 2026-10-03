package metric

import (
	"context"
	"math/rand/v2"
	"runtime"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

var _ cases.DataProvider = (*Agent)(nil)

type Agent struct{}

func NewAgent() *Agent {
	return &Agent{}
}

func (a *Agent) GetActualAgentData(ctx context.Context) (*entities.Metrics, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "get actual agent data. context")
	}

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	metrics := entities.NewMetrics()
	values := []struct {
		name  entities.MName
		value float64
	}{
		{name: entities.Alloc, value: float64(stats.Alloc)},
		{name: entities.BuckHashSys, value: float64(stats.BuckHashSys)},
		{name: entities.Frees, value: float64(stats.Frees)},
		{name: entities.GCCPUFraction, value: stats.GCCPUFraction},
		{name: entities.GCSys, value: float64(stats.GCSys)},
		{name: entities.HeapAlloc, value: float64(stats.HeapAlloc)},
		{name: entities.HeapIdle, value: float64(stats.HeapIdle)},
		{name: entities.HeapInuse, value: float64(stats.HeapInuse)},
		{name: entities.HeapObjects, value: float64(stats.HeapObjects)},
		{name: entities.HeapReleased, value: float64(stats.HeapReleased)},
		{name: entities.HeapSys, value: float64(stats.HeapSys)},
		{name: entities.LastGC, value: float64(stats.LastGC)},
		{name: entities.Lookups, value: float64(stats.Lookups)},
		{name: entities.MCacheInuse, value: float64(stats.MCacheInuse)},
		{name: entities.MCacheSys, value: float64(stats.MCacheSys)},
		{name: entities.MSpanInuse, value: float64(stats.MSpanInuse)},
		{name: entities.MSpanSys, value: float64(stats.MSpanSys)},
		{name: entities.Mallocs, value: float64(stats.Mallocs)},
		{name: entities.NextGC, value: float64(stats.NextGC)},
		{name: entities.NumForcedGC, value: float64(stats.NumForcedGC)},
		{name: entities.NumGC, value: float64(stats.NumGC)},
		{name: entities.OtherSys, value: float64(stats.OtherSys)},
		{name: entities.PauseTotalNs, value: float64(stats.PauseTotalNs)},
		{name: entities.StackInuse, value: float64(stats.StackInuse)},
		{name: entities.StackSys, value: float64(stats.StackSys)},
		{name: entities.Sys, value: float64(stats.Sys)},
		{name: entities.TotalAlloc, value: float64(stats.TotalAlloc)},
		{name: entities.RandomValue, value: rand.Float64()},
	}

	for _, metric := range values {
		if err := metrics.UpdateGauge(metric.name, metric.value); err != nil {
			return nil, errors.Wrap(entities.ErrInternalError, "get actual agent data. update gauge")
		}
	}

	return metrics, nil
}
