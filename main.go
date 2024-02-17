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
)

var (
	cacheNop             bool
	closeDocChan         chan Document
	js                   jetstream.JetStream
	logger               *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	nc                   *nats.Conn
	saveExtractedDocChan chan *ExtractedDocument
	srv                  http.Server
)

func main() {
	conf := NewTesConigFromEnv()
	closeDocChan = make(chan Document, 100)
	saveExtractedDocChan = make(chan *ExtractedDocument, 10)
	go saveAndCloseExtracedDocs()

	router := gin.New()
	router.Use(sloggin.New(logger), gin.Recovery())
	router.POST("/", ExtractBody)
	router.GET("/", ExtractRemote)
	router.HEAD("/", ExtractRemote)
	router.GET("/debug/vars", expvar.Handler())

	srv.Addr = conf.SrvAddr
	srv.Handler = router
	if conf.Debug {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Info("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}
	buildinfo, _ := debug.ReadBuildInfo()
	logger.Debug("Info", "buildinfo", buildinfo)

	var err error

	if conf.NatsUrl != "" {
		logger.Info("Connecting to NATS", "server", conf.NatsUrl)
		nc, err = nats.Connect(conf.NatsUrl)
		if err != nil {
			panic(err)
		}
	} else if !cacheNop {
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
		initCache(conf.Bucket, conf.Replicas)
		defer nc.Drain()
	} else {
		logger.Info("Cache disabled.")
	}

	if nc != nil {
		RegisterNatsService()
	}

	if conf.NoHttp {
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
