//go:build !cache_nop

package main

import (
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// SetupNatsConnection connects the service with NATS.
// Depending on the config an embedded NATS server is started
func SetupNatsConnection(conf TesConfig) (*nats.Conn, jetstream.JetStream) {
	var js jetstream.JetStream
	var nc *nats.Conn
	var err error
	if conf.NatsUrl != "" {
		logger.Info("Connecting to NATS", "server", conf.NatsUrl)
		nc, err = nats.Connect(conf.NatsUrl)
		if err != nil {
			panic(err)
		}
	} else {
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
		if !ns.ReadyForConnections(3 * time.Second) {
			panic("Nats not ready!")
		}

		if err != nil {
			panic(err)
		}
		nc, err = nats.Connect(ns.ClientURL(),
			// connect in-process rather then per TCP
			func(o *nats.Options) error {
				o.InProcessServer = ns
				return nil
			})
		if err != nil {
			panic(err)
		}
	}

	js, err = jetstream.New(nc)
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Info("NATS server connected. JetStream enabled.")

	return nc, js
}
