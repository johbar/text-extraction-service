//go:build !cache_nop

package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// SetupNatsConnection connects the service to NATS.
// Depending on the config an embedded NATS server is started.
func SetupNatsConnection(conf TesConfig) (*nats.Conn, jetstream.JetStream) {
	var js jetstream.JetStream
	var nc *nats.Conn
	var err error
	var attempts int = 0
	if conf.NatsUrl != "" {
		logger.Info("Try connecting to NATS", "url", conf.NatsUrl, "timeoutSecs", conf.NatsTimeout.Seconds(), "count", attempts)
		for nc == nil {
			attempts++
			nc, err = nats.Connect(conf.NatsUrl, nats.Name("TES"), nats.Timeout(conf.NatsTimeout))
			if err != nil {
				logger.Error("Connecting to NATS failed",
					"url", conf.NatsUrl,
					"timeoutSecs", conf.NatsTimeout.Seconds(),
					"err", err,
					"count", attempts,
					"maxRetries", conf.NatsConnectRetries)
				if attempts > conf.NatsConnectRetries {
					logger.Error("Connecting to NATS failed. Retry count exceeded", "err", err, "maxRetries", conf.NatsConnectRetries)
					if conf.FailWithoutJetstream {
						logger.Error("FATAL: terminating")
						os.Exit(2)
					} else {
						logger.Warn("Resuming initialization without NATS. Cache disabled.")
						return nil, nil
					}
				}
				time.Sleep(time.Second)
			} else {
				logger.Info("NATS connected")
			}
		}
	} else {
		nc, err = connectToEmbeddedNatsServer(conf)
		if err != nil {
			return nil, nil
		}
	}

	js, err = jetstream.New(nc)
	if err != nil {
		logger.Error("FATAL: Error when initializing NATS JetStream", "err", err.Error())
		os.Exit(1)
	}
	// test if JetStream is available
	// we reuse the retry attempt counter from above
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = js.AccountInfo(ctx)
		if err != nil {
			if errors.Is(err, jetstream.ErrJetStreamNotEnabled) || errors.Is(err, jetstream.ErrJetStreamNotEnabledForAccount) {
				logger.Error("FATAL: JetStream not enabled or not enabled for this account.")
				os.Exit(2)
			}
			logger.Error("NATS JetStream check failed. Is JetStream enabled in external NATS server(s)?",
				"err", err,
				"count", attempts,
				"maxRetries", conf.NatsConnectRetries)
			if attempts >= conf.NatsConnectRetries {
				if conf.FailWithoutJetstream {
					os.Exit(2)
				} else {
					break
				}
			}
			attempts++
			time.Sleep(time.Second)
		} else {
			break
		}
	}
	return nc, js
}
