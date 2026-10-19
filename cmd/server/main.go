package main

import (
	"log"
	"os"

	"github.com/pkg/errors"

	"github.com/parta4ok/yandex-metrics/metrics/pkg/application"
)

func main() {
	if err := run(os.Args[1:], os.Environ()); err != nil {
		log.Fatalf("%+v", err)
	}
}

func run(args []string, environ []string) error {
	app, err := application.New(args, environ)
	if err != nil {
		return errors.Wrap(err, "run metrics service. create application")
	}

	return app.Run()
}
