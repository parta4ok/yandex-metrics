package noop

import "context"

type Backup[T any] struct{}

func New[T any]() *Backup[T] {
	return &Backup[T]{}
}

func (b *Backup[T]) MakeBackup(_ context.Context, _ []T) error {
	return nil
}

func (b *Backup[T]) RestoreBackup(_ context.Context) ([]T, error) {
	return nil, nil
}
