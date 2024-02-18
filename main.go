package main

import (
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/gin-contrib/expvar"
	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
)

var (
	cache                Cache
	cacheNop             bool
	closeDocChan         chan Document
	logger               *slog.Logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
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

	nc, js := SetupNatsConnection(conf)
	if nc != nil {
		cache = InitCache(js, conf.Bucket, conf.Replicas)
		RegisterNatsService(nc)
	}
	defer nc.Drain()


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
