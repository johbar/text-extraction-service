package main

import (
	"time"

	"go-simpler.org/env"
)

// TesConfig represents the configuration of this service
type TesConfig struct {
	// Name of the object store or key-value bucket in NATS to use
	// Default: TES_PLAINTEXTS
	Bucket string `env:"TES_BUCKET" default:"TES_PLAINTEXTS"`
	// wether to expose embedded NATS server to other clients. Default: false
	ExposeNats bool `env:"TES_EXPOSE_NATS" default:"false"`
	// increase log level (debug instead of info). Default: false
	Debug bool `env:"TES_DEBUG" default:"false"`
	// If true the service will exit with an error if NATS or JetStream can't be connected
	FailWithoutJetstream bool `env:"TES_FAIL_WITHOUT_JS" default:"true"`
	// NATS max msg size (embedded server only)
	NatsMaxPayload int32 `env:"TES_MAX_PAYLOAD" default:"8388608"`
	// embedded NATS server storage location. Default: /tmp/nats
	NatsStoreDir string `env:"TES_NATS_STORE_DIR"`
	// embedded NATS server host/ip address, if exposed. Default: localhost
	NatsHost string `env:"TES_NATS_HOST" default:"localhost"`
	// embedded NATS server port, if exposed. Default: 4222
	NatsPort int `env:"TES_NATS_PORT" default:"4222"`
	// External NATS URL, e.g. nats://localhost:4222
	NatsUrl string `env:"TES_NATS_URL"`
	// Timeout for the external NATS connection
	NatsTimeout time.Duration `env:"TES_NATS_TIMEOUT" default:"15s"`
	// NatsConnectRetries is the number of attempts to connect to external NATS server(s)
	NatsConnectRetries int `env:"TES_NATS_CONNECT_RETRIES" default:"10"`
	// if true, disable HTTP Server in favor of NATS Microservice interface
	NoHttp bool `env:"TES_NO_HTTP" default:"false"`
	// How many replicas of the bucket to create. Default: 1
	Replicas int `env:"TES_REPLICAS" default:"1"`
	// HTTP listen address and/or port. Default: ':8080'
	SrvAddr string `host_port:"TES_HOST_PORT" default:":8080"`
}

// NewTesConigFromEnv returns a service config object
// populated with defaults and values from environment vars
func NewTesConigFromEnv() TesConfig {
	var cfg TesConfig
	if err := env.Load(&cfg, nil); err != nil {
		logger.Error("Loading config failed", "err", err)
	}

	return cfg
}
