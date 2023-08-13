package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/gin-contrib/expvar"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	sloggin "github.com/samber/slog-gin"
)

var (
	closeDocChan         chan Document
	saveExtractedDocChan chan *ExtractedDocument
	srv                  http.Server
	nc                   *nats.Conn
	plaintextBucket      nats.KeyValue
	metadataBucket       nats.KeyValue

	logger *slog.Logger = slog.Default()
)

const (
	// config items
	confHostPort   = "host_port"
	confMaxPayload = "max_payload"
	confExposeNats = "expose_nats"
	confNatsPort   = "nats_port"
	confNatsHost   = "nats_host"
	confExtNats    = "external_nats"
	confNatsDir    = "nats_store_dir"
)

func main() {
	// log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Info("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}
	// buildinfo, _ := debug.ReadBuildInfo()
	// log.Printf("%v", buildinfo)
	closeDocChan = make(chan Document, 100)
	saveExtractedDocChan = make(chan *ExtractedDocument, 10)
	go saveAndCloseExtracedDocs()
	router := gin.New()
	router.Use(sloggin.New(logger), gin.Recovery())
	router.POST("/pdf", ExtractBody)
	router.GET("/pdf", ExtractRemote)
	router.HEAD("/pdf", ExtractRemote)
	router.GET("/debug/vars", expvar.Handler())
	// router.GET("/json", ExtractAsJson)
	// router.GET("/pdf/forget", ExtractRemoteAsync)
	// wg := startWorkers()

	viper.SetEnvPrefix("tes")
	// This service
	viper.SetDefault(confHostPort, ":8080")
	viper.SetDefault(confMaxPayload, 8*1024*1024)
	viper.SetDefault(confExposeNats, false)
	viper.SetDefault(confNatsPort, 4222)
	viper.SetDefault(confNatsHost, "localhost")
	viper.SetDefault(confExtNats, false)

	viper.AutomaticEnv()

	srv.Addr = viper.GetString(confHostPort)
	srv.Handler = router
	maxPayload := viper.GetInt32(confMaxPayload)
	natsHost := viper.GetString(confNatsHost)
	natsPort := viper.GetInt(confNatsPort)

	useExtNats := viper.GetBool(confExtNats)
	var err error
	if !useExtNats {
		ns, err := server.NewServer(
			&server.Options{
				// Host:               "localhost",
				JetStream:          true,
				MaxPayload:         maxPayload,
				JetStreamMaxMemory: 1024 * 1000,
				TLS:                false,
				DontListen:         !viper.GetBool("expose_nats"),
				Host:               natsHost,
				Port:               natsPort,
				StoreDir:           viper.GetString(confNatsDir),
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
	} else {
		connStr := fmt.Sprintf("nats://%s:%d", natsHost, natsPort)
		logger.Info("Connecting to Nats", "server", connStr)
		nc, err = nats.Connect(connStr)
		if err != nil {
			panic(err)
		}
	}
	js, err := nc.JetStream()
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Info("NATS server connected. JetStream enabled.")

	kvPlainTexts := &nats.KeyValueConfig{Bucket: "plaintexts_zstd", MaxValueSize: maxPayload, Storage: nats.FileStorage}
	kvMetaConf := &nats.KeyValueConfig{Bucket: "metadata", MaxValueSize: maxPayload, Storage: nats.FileStorage}
	plaintextBucket, err = js.CreateKeyValue(kvPlainTexts)
	if err != nil {
		logger.Error(err.Error())
	}
	metadataBucket, err = js.CreateKeyValue(kvMetaConf)
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Info("Service started", "address", srv.Addr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("Weberserver", "error", err)
	}
	logger.Info("Nats server still running...")
	logger.Info("HTTP Server stopped.")

}
