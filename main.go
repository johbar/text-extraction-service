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
    "github.com/nats-io/nats.go/jetstream"
	sloggin "github.com/samber/slog-gin"
	"github.com/spf13/viper"
)

var (
	cacheNop             bool
	closeDocChan         chan Document
	js                   jetstream.JetStream
	logger               *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	maxPayload           int32
	nc                   *nats.Conn
	saveExtractedDocChan chan *ExtractedDocument
	srv                  http.Server
)

const (
	// config items
	confExposeNats = "expose_nats"
	confExtNats    = "external_nats"
	confHostPort   = "host_port"
	confMaxPayload = "max_payload"
	confNatsDir    = "nats_store_dir"
	confNatsHost   = "nats_host"
	confNatsPort   = "nats_port"
)

func main() {
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
	router.POST("/", ExtractBody)
	router.GET("/", ExtractRemote)
	router.HEAD("/", ExtractRemote)
	router.GET("/debug/vars", expvar.Handler())

	viper.SetEnvPrefix("tes")
	viper.SetDefault(confHostPort, ":8080")
	viper.SetDefault(confMaxPayload, 10 * 1024*1024)
	viper.SetDefault(confExposeNats, false)
	viper.SetDefault(confNatsPort, 4222)
	viper.SetDefault(confNatsHost, "localhost")
	viper.SetDefault(confExtNats, false)

	viper.AutomaticEnv()

	srv.Addr = viper.GetString(confHostPort)
	srv.Handler = router
	maxPayload = viper.GetInt32(confMaxPayload)
	natsHost := viper.GetString(confNatsHost)
	natsPort := viper.GetInt(confNatsPort)

	useExtNats := viper.GetBool(confExtNats)
	var err error

	if useExtNats {
		connStr := fmt.Sprintf("nats://%s:%d", natsHost, natsPort)
		logger.Info("Connecting to Nats", "server", connStr)
		nc, err = nats.Connect(connStr)
		if err != nil {
			panic(err)
		}
	} else if !cacheNop {
		ns, err := server.NewServer(
			&server.Options{
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
	}
	if !cacheNop {
		js, err = jetstream.New(nc)
		if err != nil {
			logger.Error(err.Error())
		}
		logger.Info("NATS server connected. JetStream enabled.")
		initCache()
	} else {
		logger.Info("Cache disabled.")
	}

	logger.Info("Service started", "address", srv.Addr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("Webserver", "error", err)
	}
	logger.Info("Nats server still running...")
	logger.Info("HTTP Server stopped.")

}
