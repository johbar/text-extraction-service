//go:build !embed_nats

package nats

import (
	tesconfig "github.com/johbar/text-extraction-service/v4/internal/config"
	"github.com/nats-io/nats.go"
)

const NatsEmbedded bool = false

func ConnectToEmbeddedNatsServer(_ tesconfig.TesConfig) (*nats.Conn, error) {
	return nil, errNatsNotEmbedded
}
