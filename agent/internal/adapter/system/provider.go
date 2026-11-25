package system

import (
	"context"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/parta4ok/yandex-metrics/agent/internal/cases"
	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
)

var _ cases.SystemDataProvider = (*Provider)(nil)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) GetSystemData(ctx context.Context) (*entities.Metrics, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrapf(entities.ErrInternalError, "get system data. context: %v", err)
	}

	memory, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return nil, errors.Wrapf(entities.ErrInternalError, "get system data. virtual memory: %v", err)
	}

	utilization, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		return nil, errors.Wrapf(entities.ErrInternalError, "get system data. cpu utilization: %v", err)
	}

	metrics := entities.NewMetrics()
	if err := metrics.UpdateGauge(entities.TotalMemory, float64(memory.Total)); err != nil {
		return nil, errors.Wrap(err, "get system data. update total memory")
	}
	if err := metrics.UpdateGauge(entities.FreeMemory, float64(memory.Free)); err != nil {
		return nil, errors.Wrap(err, "get system data. update free memory")
	}

	for i, value := range utilization {
		name, err := entities.NewCPUUtilization(i + 1)
		if err != nil {
			return nil, errors.Wrap(err, "get system data. cpu utilization name")
		}
		if err := metrics.UpdateGauge(name, value); err != nil {
			return nil, errors.Wrap(err, "get system data. update cpu utilization")
		}
	}

	return metrics, nil
}
