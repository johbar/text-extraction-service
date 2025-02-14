//go:build !no_embedded_nats

package main

import (
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func connectToEmbeddedNatsServer(conf TesConfig) (*nats.Conn, error) {
	ns, err := server.NewServer(
		&server.Options{
			JetStream:  true,
			MaxPayload: conf.NatsMaxPayload,
			TLS:        false,
			DontListen: !conf.ExposeNats,
			Host:       conf.NatsHost,
			Port:       conf.NatsPort,
			StoreDir:   conf.NatsStoreDir,
		})
	if err != nil {
		panic(err)
	}
	ns.ConfigureLogger()
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		panic("NATS not ready!")
	}

	return nats.Connect("", nats.InProcessServer(ns))
}
