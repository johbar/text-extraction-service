//go:build cache_nop

package main

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func init() {
	cacheNop = true
}

func InitCache(js jetstream.JetStream, conf TesConfig) Cache {
	return NopCache{}
}

// No-op NATS server connection
func SetupNatsConnection(conf TesConfig) (*nats.Conn, jetstream.JetStream) {
	return nil, nil
}
