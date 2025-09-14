//go:build embed_nats

package nats

import (
	"errors"
	"time"

	tesconfig "github.com/johbar/text-extraction-service/v4/internal/config"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

const NatsEmbedded bool = true

func ConnectToEmbeddedNatsServer(conf tesconfig.TesConfig) (*nats.Conn, error) {
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
		return nil, err
	}
	ns.ConfigureLogger()
	ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		return nil, errors.New("embedded NATS not ready")
	}

	return nats.Connect("", nats.InProcessServer(ns))
}
