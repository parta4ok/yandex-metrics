package application

import "context"

type AgentServiceProvider interface {
	UpdateMetrics(ctx context.Context) error
	SendMetrics(ctx context.Context) error
}
