package nats

import (
	"errors"
	"log/slog"
	"time"

	"github.com/johbar/text-extraction-service/v2/internal/config"
	"github.com/nats-io/nats.go"
)

var errNatsNotEmbedded = errors.New("NATS has not been embedded in this build")

// SetupNatsConnection connects the service to NATS.
// Depending on the config an embedded NATS server is started.
func SetupNatsConnection(conf config.TesConfig, log *slog.Logger) (*nats.Conn, error) {
	var nc *nats.Conn
	var err error
	var attempts int = 0

	log.Info("Try connecting to NATS", "url", conf.NatsUrl, "timeoutSecs", conf.NatsTimeout.Seconds(), "count", attempts)
	for nc == nil {
		attempts++
		nc, err = nats.Connect(conf.NatsUrl, nats.Name("TES"), nats.Timeout(conf.NatsTimeout))
		if err != nil {
			log.Error("Connecting to NATS failed",
				"url", conf.NatsUrl,
				"timeoutSecs", conf.NatsTimeout.Seconds(),
				"err", err,
				"count", attempts,
				"maxRetries", conf.NatsConnectRetries)
			if attempts > conf.NatsConnectRetries {
				log.Error("Connecting to NATS failed. Retry count exceeded", "err", err, "maxRetries", conf.NatsConnectRetries)
				return nil, err
			}
			time.Sleep(time.Second)
		} else {
			return nc, nil
		}
	}

	return nc, err
}
