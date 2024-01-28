package main

import (
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
	// config item names

	// Name of the object store or key-value bucket to use
	confBucket = "bucket"
	// wether to expose embedded Nats server to other clients
	confExposeNats = "expose_nats"
	// HTTP listen address and port
	confHostPort = "host_port"
	// increase log level
	confLogLevel   = "debug"
	confNatsConfig = "nats_config"
	// Nats max msg size (embedded server only)
	confMaxPayload = "max_payload"
	// embedded Nats server storage location
	confNatsDir = "nats_store_dir"
	// embedded Nats server Host and Port, if exposed
	confNatsHost = "nats_host"
	confNatsPort = "nats_port"
	// External Nats URL
	confNatsUrl = "nats_url"
	// Disable HTTP Server in favor of Nats Microservice interface
	confNoHttp = "no_http"
)

func main() {
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
	viper.SetDefault(confMaxPayload, 10*1024*1024)
	viper.SetDefault(confExposeNats, false)
	viper.SetDefault(confNatsPort, 4222)
	viper.SetDefault(confNatsHost, "localhost")
	viper.SetDefault(confNoHttp, false)
	viper.SetDefault(confLogLevel, false)
	viper.SetDefault(confBucket, "TES_PLAINTEXTS")
	// viper.SetDefault(nonfCo)

	viper.AutomaticEnv()
	if viper.GetBool("debug") {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Info("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}
	buildinfo, _ := debug.ReadBuildInfo()
	logger.Debug("Info", "buildinfo", buildinfo)
	srv.Addr = viper.GetString(confHostPort)
	srv.Handler = router
	maxPayload = viper.GetInt32(confMaxPayload)
	natsHost := viper.GetString(confNatsHost)
	natsPort := viper.GetInt(confNatsPort)

	useExtNats := viper.IsSet(confNatsUrl)
	var err error

	if useExtNats {
		connStr := viper.GetString(confNatsUrl)
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
		defer ns.Shutdown()
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
		initCache(viper.GetString(confBucket))
		defer nc.Drain()
	} else {
		logger.Info("Cache disabled.")
	}

	if nc != nil {
		RegisterNatsService()
	}

	if viper.GetBool(confNoHttp) {
		if nc == nil {
			logger.Error("Fatal: NATS not connected and HTTP disabled.")
			os.Exit(1)
		}
		wait := make(chan bool, 1)
		logger.Info("Service started with no HTTP endpoints. Waiting for interrupt.")
		<-wait
	}
	logger.Info("Service started", "address", srv.Addr)
	defer logger.Info("HTTP Server stopped.")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("Webserver", "error", err)
	}

}
