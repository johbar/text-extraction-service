package main

import "github.com/spf13/viper"

const (
	// config item names, uppercased variants with TES_ Prefix
	// = environment vars
	confBucket     = "bucket"
	confReplicas   = "replicas"
	confExposeNats = "expose_nats"
	confHostPort   = "host_port"
	confLogLevel   = "debug"
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
	Bucket string
	// wether to expose embedded NATS server to other clients
	ExposeNats bool
	// increase log level
	Debug bool
	// NATS max msg size (embedded server only)
	NatsMaxPayload int32
	// embedded NATS server storage location
	NatsStoreDir string
	// embedded NATS server host/ip address, if exposed
	NatsHost string
	// embedded NATS server port, if exposed
	NatsPort int
	// External NATS URL
	NatsUrl string
	// if true, disable HTTP Server in favor of NATS Microservice interface
	NoHttp bool
	// How many replicas of the bucket to create
	Replicas int
	// HTTP listen address and port
	SrvAddr string
}

// NewTesConigFromEnv returns a service config object
// populated with defaults and values from environment vars
func NewTesConigFromEnv() TesConfig {
	viper.SetEnvPrefix("tes")
	viper.SetDefault(confHostPort, ":8080")
	viper.SetDefault(confMaxPayload, 10*1024*1024)
	viper.SetDefault(confExposeNats, false)
	viper.SetDefault(confNatsPort, 4222)
	viper.SetDefault(confNatsHost, "localhost")
	viper.SetDefault(confNoHttp, false)
	viper.SetDefault(confLogLevel, false)
	viper.SetDefault(confBucket, "TES_PLAINTEXTS")
	viper.SetDefault(confReplicas, 1)

	return TesConfig{
		Bucket:         viper.GetString(confBucket),
		ExposeNats:     viper.GetBool("expose_nats"),
		Debug:          viper.GetBool("debug"),
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
