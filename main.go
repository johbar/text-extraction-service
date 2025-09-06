package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"

	"github.com/johbar/text-extraction-service/v2/internal/cache"
	tesnats "github.com/johbar/text-extraction-service/v2/internal/cache/nats"
	"github.com/johbar/text-extraction-service/v2/internal/config"
	"github.com/johbar/text-extraction-service/v2/internal/docfactory"
	"github.com/johbar/text-extraction-service/v2/internal/extractor"
	"github.com/nats-io/nats.go"

	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

var (
	log *slog.Logger
	srv http.Server
)

func main() {
	var err error
	tesConfig, err := config.NewTesConfigFromEnv()
	if err != nil {
		slog.Error("FATAL: error when parsing config values", "err", err)
	}

	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{})

	log = slog.New(h)
	// set static/global config of submodules
	tesswrap.Languages = tesConfig.TesseractLangs
	dehyphenator.RemoveNewlines = tesConfig.RemoveNewlines
	docFactory := docfactory.New(tesConfig, log)

	var tesCache cache.Cache
	var objStore *cache.ObjectStoreCache
	var nc *nats.Conn
	/*
		There are 4 cases:
		1. A NatsUrl is configured. -> connect
			a. If that fails and FailWithoutJetstream == true, fail the program start.
			b. If that fails and FailWithoutJetstream == false, continue with NopCache.
		2. No NatsUrl is configured but NATS is embedded -> connect.
	*/
	natsConfigured := len(tesConfig.NatsUrl) > 0
	if natsConfigured {
		// case 1
		nc, err = tesnats.SetupNatsConnection(*tesConfig, log)
	} else if tesnats.NatsEmbedded {
		// case 2
		nc, err = tesnats.ConnectToEmbeddedNatsServer(*tesConfig)
	}
	if err != nil {
		log.Error("connecting to NATS failed", "err", err)
	} else {
		objStore, err = cache.New(*tesConfig, log, nc)
		if err != nil {
			if tesConfig.FailWithoutJetstream && natsConfigured {
				// case 1a
				log.Error("FATAL: could not init NATS based cache and TES_FAIL_WITHOUT_JS is true", "err", err)
				os.Exit(2)
			}
			if tesConfig.NoHttp {
				log.Error("FATAL: NATS not connected and HTTP disabled.", "err", err)
				os.Exit(1)
			}
		}
	}
	if objStore == nil {
		// case 1b
		tesCache = &cache.NopCache{}
	} else {
		tesCache = objStore
	}

	httpClient := &http.Client{
		Transport: &http.Transport{DisableCompression: tesConfig.HttpClientDisableCompression, MaxIdleConnsPerHost: 100},
	}
	extr := extractor.New(tesConfig, docFactory, tesCache, log, httpClient)
	extr.LogAndFixConfigIssues()
	// one shot mode: don't start a server, just process a single file provided on the command line
	if len(os.Args) > 1 {
		// logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
		extr.PrintMetadataAndTextToStdout(os.Args[1])
		return
	}

	log.Debug("Starting Text Extraction Service with config", "conf", tesConfig)
	if tesConfig.NoHttp {
		wait := make(chan struct{})
		log.Info("Service started with no HTTP endpoints. Waiting for interrupt.")
		<-wait
	}
	router := chi.NewRouter()

	router.Use(httplog.RequestLogger(&httplog.Logger{
		Logger: log,
		Options: httplog.Options{
			Concise: true,
		},
	}), middleware.Recoverer)
	router.Post("/", extr.ExtractBody)
	router.Get("/", extr.ExtractRemote)
	router.Head("/", extr.ExtractRemote)
	// FIXME: do we need this?
	// router.GET("/debug/vars", expvar.Handler())

	srv.Addr = tesConfig.SrvAddr
	srv.Handler = router

	log.Info("Service started", "address", srv.Addr)
	defer log.Info("HTTP Server stopped.")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Error("Webserver failed", "err", err)
	}
}
