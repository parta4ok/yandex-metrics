package config

import "strings"

const (
	addressEnv         = "ADDRESS"
	storeIntervalEnv   = "STORE_INTERVAL"
	fileStoragePathEnv = "FILE_STORAGE_PATH"
	restoreEnv         = "RESTORE"
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
