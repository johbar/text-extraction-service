package main

import "github.com/spf13/viper"

const (
	// config item names, uppercased variants with TES_ prefix
	// = environment vars
	confBucket     = "bucket"
	confReplicas   = "replicas"
	confExposeNats = "expose_nats"
	confHostPort   = "host_port"
	confDebug      = "debug"
	confMaxPayload = "max_payload"
	confNatsDir    = "nats_store_dir"
	confNatsHost   = "nats_host"
	confNatsPort   = "nats_port"
	confNatsUrl    = "nats_url"
	confNoHttp     = "no_http"
)

// TesConfig represents the configuration of this service
type TesConfig struct {
	// Name of the object store or key-value bucket in NATS to use
	// Default: TES_PLAINTEXTS
	Bucket string
	// wether to expose embedded NATS server to other clients. Default: false
	ExposeNats bool
	// increase log level (debug instead of info). Default: false
	Debug bool
	// NATS max msg size (embedded server only)
	NatsMaxPayload int32
	// embedded NATS server storage location. Default: /tmp/nats
	NatsStoreDir string
	// embedded NATS server host/ip address, if exposed. Default: localhost
	NatsHost string
	// embedded NATS server port, if exposed. Default: 4222
	NatsPort int
	// External NATS URL, e.g. nats://localhost:4222
	NatsUrl string
	// if true, disable HTTP Server in favor of NATS Microservice interface
	NoHttp bool
	// How many replicas of the bucket to create. Default: 1
	Replicas int
	// HTTP listen address and/or port. Default: ':8080'
	SrvAddr string
}

// NewTesConigFromEnv returns a service config object
// populated with defaults and values from environment vars
func NewTesConigFromEnv() TesConfig {
	viper.SetEnvPrefix("tes")
	viper.AutomaticEnv()
	viper.SetDefault(confHostPort, ":8080")
	viper.SetDefault(confMaxPayload, 10*1024*1024)
	viper.SetDefault(confExposeNats, false)
	viper.SetDefault(confNatsPort, 4222)
	viper.SetDefault(confNatsHost, "localhost")
	viper.SetDefault(confNoHttp, false)
	viper.SetDefault(confDebug, false)
	viper.SetDefault(confBucket, "TES_PLAINTEXTS")
	viper.SetDefault(confReplicas, 1)

	return TesConfig{
		Bucket:         viper.GetString(confBucket),
		ExposeNats:     viper.GetBool(confExposeNats),
		Debug:          viper.GetBool(confDebug),
		NatsMaxPayload: viper.GetInt32(confMaxPayload),
		NatsStoreDir:   viper.GetString(confNatsDir),
		NatsHost:       viper.GetString(confNatsHost),
		NatsPort:       viper.GetInt(confNatsPort),
		NatsUrl:        viper.GetString(confNatsUrl),
		NoHttp:         viper.GetBool(confNoHttp),
		Replicas:       viper.GetInt(confReplicas),
		SrvAddr:        viper.GetString(confHostPort),
	}
}
