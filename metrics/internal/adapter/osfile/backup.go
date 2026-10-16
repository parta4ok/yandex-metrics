package osfile

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/internal/cases"
	"github.com/parta4ok/yandex-metrics/metrics/internal/entities"
	toolkitlogger "github.com/parta4ok/yandex-metrics/toolkit/logger"
)

var _ cases.BackupProvider = (*Backup)(nil)

type record struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  *string  `json:"hash,omitempty"`
}

type Backup struct {
	mu     sync.Mutex
	path   string
	logger toolkitlogger.Logger
}

func NewBackup(path string, logger toolkitlogger.Logger) (*Backup, error) {
	if path == "" {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup. path is empty")
	}
	if logger == nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "new backup. logger is nil")
	}

	return &Backup{
		path:   path,
		logger: logger,
	}, nil
}

func (b *Backup) MakeBackup(ctx context.Context, metrics []*entities.Metric) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "make backup. context")
	}

	records := make([]record, 0, len(metrics))
	for _, metric := range metrics {
		records = append(records, record{
			ID:    metric.ID(),
			MType: string(metric.MType()),
			Delta: metric.Delta(),
			Value: metric.Value(),
			Hash:  metric.Hash(),
		})
	}

	data, err := json.Marshal(records)
	if err != nil {
		return errors.Wrap(err, "make backup. marshal records")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.writeFile(data); err != nil {
		return errors.Wrap(err, "make backup. write file")
	}

	return nil
}

func (b *Backup) RestoreBackup(ctx context.Context) ([]*entities.Metric, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "restore backup. context")
	}

	b.mu.Lock()
	data, err := os.ReadFile(b.path)
	b.mu.Unlock()
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "restore backup. read file")
	}
	if len(data) == 0 {
		return nil, nil
	}

	var records []record
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, errors.Wrap(entities.ErrInvalidParam, "restore backup. backup file is invalid")
	}

	metrics := make([]*entities.Metric, 0, len(records))
	for _, record := range records {
		metric, err := entities.NewMetric(record.ID, entities.MType(record.MType))
		if err != nil {
			return nil, errors.Wrap(err, "restore backup. create metric")
		}
		metric.SetDelta(record.Delta)
		metric.SetValue(record.Value)
		metric.SetHash(record.Hash)

		metrics = append(metrics, metric)
	}

	return metrics, nil
}

func (b *Backup) writeFile(data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(b.path), filepath.Base(b.path)+".tmp-*")
	if err != nil {
		return errors.Wrap(err, "write file. create temp file")
	}
	defer os.Remove(temp.Name())

	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return errors.Wrap(err, "write file. write temp file")
	}
	if err := temp.Close(); err != nil {
		return errors.Wrap(err, "write file. close temp file")
	}
	if err := os.Rename(temp.Name(), b.path); err != nil {
		return errors.Wrap(err, "write file. rename temp file")
	}

	return nil
}
