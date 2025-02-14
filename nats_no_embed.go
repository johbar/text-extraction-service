//go:build no_embedded_nats

package main

import (
	"errors"

	"github.com/nats-io/nats.go"
)

func connectToEmbeddedNatsServer(conf TesConfig) (*nats.Conn, error) {
	return nil, errors.New("NATS has not been embedded in this build")
}
