package config

import "strings"

const (
	addressEnv        = "ADDRESS"
	reportIntervalEnv = "REPORT_INTERVAL"
	pollIntervalEnv   = "POLL_INTERVAL"
	signatureKeyEnv   = "KEY"
	rateLimitEnv      = "RATE_LIMIT"
)

type environ map[string]string

func newEnviron(list []string) environ {
	values := make(environ, len(list))
	for _, item := range list {
		if name, value, ok := strings.Cut(item, "="); ok {
			values[name] = value
		}
	}

	return values
}

func (e environ) lookup(name string) (string, bool) {
	value, ok := e[name]

	return value, ok
}
