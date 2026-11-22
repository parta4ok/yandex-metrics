package entities_test

import (
	"testing"

	"github.com/parta4ok/yandex-metrics/agent/internal/entities"
	"github.com/stretchr/testify/require"
)

func TestMType_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		mType entities.MType
		want  bool
	}{
		{name: "counter", mType: entities.Counter, want: true},
		{name: "gauge", mType: entities.Gauge, want: true},
		{name: "invalid", mType: "invalid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, tt.mType.IsValid())
		})
	}
}

func TestMName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		metricName entities.MName
		valid      bool
		mType      entities.MType
	}{
		{name: "runtime gauge", metricName: entities.Alloc, valid: true, mType: entities.Gauge},
		{name: "counter", metricName: entities.PollCount, valid: true, mType: entities.Counter},
		{name: "random gauge", metricName: entities.RandomValue, valid: true, mType: entities.Gauge},
		{name: "total memory", metricName: entities.TotalMemory, valid: true, mType: entities.Gauge},
		{name: "free memory", metricName: entities.FreeMemory, valid: true, mType: entities.Gauge},
		{name: "first cpu", metricName: "CPUutilization1", valid: true, mType: entities.Gauge},
		{name: "many cpus", metricName: "CPUutilization128", valid: true, mType: entities.Gauge},
		{name: "cpu without index", metricName: "CPUutilization"},
		{name: "cpu zero index", metricName: "CPUutilization0"},
		{name: "cpu negative index", metricName: "CPUutilization-1"},
		{name: "cpu leading zero", metricName: "CPUutilization01"},
		{name: "cpu plus sign", metricName: "CPUutilization+1"},
		{name: "cpu not a number", metricName: "CPUutilizationX"},
		{name: "cpu with suffix", metricName: "CPUutilization1x"},
		{name: "cpu wrong case", metricName: "cpuutilization1"},
		{name: "invalid", metricName: "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.valid, tt.metricName.IsValid())
			require.Equal(t, tt.mType, tt.metricName.MType())
		})
	}
}

func TestMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		metric  *entities.Metric
		action  func(*entities.Metric) error
		wantErr error
	}{
		{
			name:    "set delta on nil metric",
			action:  func(metric *entities.Metric) error { return metric.SetDelta(1) },
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:   "set delta on gauge",
			metric: newMetric(t, entities.Alloc),
			action: func(metric *entities.Metric) error {
				return metric.SetDelta(1)
			},
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:   "set delta on counter",
			metric: newMetric(t, entities.PollCount),
			action: func(metric *entities.Metric) error {
				return metric.SetDelta(1)
			},
		},
		{
			name:    "set value on nil metric",
			action:  func(metric *entities.Metric) error { return metric.SetValue(1) },
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:   "set value on counter",
			metric: newMetric(t, entities.PollCount),
			action: func(metric *entities.Metric) error {
				return metric.SetValue(1)
			},
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:   "set value on gauge",
			metric: newMetric(t, entities.Alloc),
			action: func(metric *entities.Metric) error {
				return metric.SetValue(1)
			},
		},
		{
			name:    "increment nil metric",
			action:  func(metric *entities.Metric) error { return metric.Increment(1) },
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:   "increment gauge",
			metric: newMetric(t, entities.Alloc),
			action: func(metric *entities.Metric) error {
				return metric.Increment(1)
			},
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:   "increment counter",
			metric: newMetric(t, entities.PollCount),
			action: func(metric *entities.Metric) error {
				return metric.Increment(2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.action(tt.metric)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestNewMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		metricName entities.MName
		wantErr    error
	}{
		{name: "gauge", metricName: entities.Alloc},
		{name: "counter", metricName: entities.PollCount},
		{name: "invalid", metricName: "invalid", wantErr: entities.ErrInvalidParam},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			metric, err := entities.NewMetric(tt.metricName)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, metric)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.metricName, metric.Name())
			require.Equal(t, tt.metricName.MType(), metric.MType())
		})
	}
}

func TestMetric_ValidateAndClone(t *testing.T) {
	t.Parallel()

	counter := newMetric(t, entities.PollCount)
	require.NoError(t, counter.SetDelta(3))

	gauge := newMetric(t, entities.Alloc)
	require.NoError(t, gauge.SetValue(1.5))

	tests := []struct {
		name    string
		metric  *entities.Metric
		wantErr error
	}{
		{name: "nil metric", wantErr: entities.ErrInvalidParam},
		{name: "invalid zero metric", metric: &entities.Metric{}, wantErr: entities.ErrInvalidParam},
		{name: "counter without delta", metric: newMetric(t, entities.PollCount), wantErr: entities.ErrInvalidParam},
		{name: "gauge without value", metric: newMetric(t, entities.Alloc), wantErr: entities.ErrInvalidParam},
		{name: "valid counter", metric: counter},
		{name: "valid gauge", metric: gauge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.metric.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				clone, cloneErr := tt.metric.Clone()
				require.ErrorIs(t, cloneErr, tt.wantErr)
				require.Nil(t, clone)
				return
			}

			require.NoError(t, err)

			clone, cloneErr := tt.metric.Clone()
			require.NoError(t, cloneErr)
			require.NotSame(t, tt.metric, clone)
			require.Equal(t, tt.metric.Name(), clone.Name())
			require.Equal(t, tt.metric.MType(), clone.MType())
			require.Equal(t, tt.metric.Delta(), clone.Delta())
			require.Equal(t, tt.metric.Value(), clone.Value())
		})
	}
}

func TestMetric_Increment(t *testing.T) {
	t.Parallel()

	metric := newMetric(t, entities.PollCount)
	require.NoError(t, metric.Increment(1))
	require.NoError(t, metric.Increment(2))
	require.Equal(t, int64(3), *metric.Delta())
}

func TestMetrics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		metrics *entities.Metrics
		action  func(*entities.Metrics) error
		wantErr error
	}{
		{
			name:    "update nil metrics",
			action:  func(metrics *entities.Metrics) error { return metrics.UpdateGauge(entities.Alloc, 1) },
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:    "update counter as gauge",
			metrics: entities.NewMetrics(),
			action: func(metrics *entities.Metrics) error {
				return metrics.UpdateGauge(entities.PollCount, 1)
			},
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:    "increment nil metrics",
			action:  func(metrics *entities.Metrics) error { return metrics.IncrementCounter(entities.PollCount, 1) },
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:    "increment gauge as counter",
			metrics: entities.NewMetrics(),
			action: func(metrics *entities.Metrics) error {
				return metrics.IncrementCounter(entities.Alloc, 1)
			},
			wantErr: entities.ErrInvalidParam,
		},
		{
			name:    "update gauge",
			metrics: entities.NewMetrics(),
			action: func(metrics *entities.Metrics) error {
				return metrics.UpdateGauge(entities.Alloc, 1)
			},
		},
		{
			name:    "increment counter",
			metrics: entities.NewMetrics(),
			action: func(metrics *entities.Metrics) error {
				return metrics.IncrementCounter(entities.PollCount, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.action(tt.metrics)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestMetrics_All(t *testing.T) {
	t.Parallel()

	metrics := entities.NewMetrics()
	require.NoError(t, metrics.UpdateGauge(entities.RandomValue, 1.5))
	require.NoError(t, metrics.UpdateGauge(entities.Alloc, 2.5))
	require.NoError(t, metrics.UpdateGauge(entities.Alloc, 2.5))
	require.NoError(t, metrics.IncrementCounter(entities.PollCount, 3))
	require.NoError(t, metrics.IncrementCounter(entities.PollCount, 0))

	tests := []struct {
		name    string
		metrics *entities.Metrics
		wantErr error
	}{
		{name: "nil metrics", wantErr: entities.ErrInvalidParam},
		{name: "metrics", metrics: metrics},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			actual, err := tt.metrics.All()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, actual)
				return
			}

			require.NoError(t, err)
			require.Len(t, actual, 3)
			require.Equal(t, entities.Alloc, actual[0].Name())
			require.Equal(t, entities.PollCount, actual[1].Name())
			require.Equal(t, entities.RandomValue, actual[2].Name())

			require.NoError(t, actual[0].SetValue(9.5))
			current, err := tt.metrics.All()
			require.NoError(t, err)
			require.Equal(t, 2.5, *current[0].Value())
		})
	}
}

func newMetric(t *testing.T, name entities.MName) *entities.Metric {
	t.Helper()

	metric, err := entities.NewMetric(name)
	require.NoError(t, err)

	return metric
}

func TestNewCPUUtilization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		index   int
		want    entities.MName
		wantErr error
	}{
		{name: "first", index: 1, want: "CPUutilization1"},
		{name: "tenth", index: 10, want: "CPUutilization10"},
		{name: "zero", index: 0, wantErr: entities.ErrInvalidParam},
		{name: "negative", index: -3, wantErr: entities.ErrInvalidParam},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := entities.NewCPUUtilization(tt.index)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.True(t, got.IsValid())
			require.Equal(t, entities.Gauge, got.MType())
		})
	}
}

func TestMetrics_UpdateGaugeWithSystemNames(t *testing.T) {
	t.Parallel()

	cpu, err := entities.NewCPUUtilization(2)
	require.NoError(t, err)

	metrics := entities.NewMetrics()
	require.NoError(t, metrics.UpdateGauge(entities.TotalMemory, 100))
	require.NoError(t, metrics.UpdateGauge(entities.FreeMemory, 40))
	require.NoError(t, metrics.UpdateGauge(cpu, 12.5))

	all, err := metrics.All()
	require.NoError(t, err)
	require.Len(t, all, 3)
}
