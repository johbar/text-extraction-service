package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"github.com/gin-contrib/expvar"
	"github.com/gin-gonic/gin"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
	sloggin "github.com/samber/slog-gin"
)

var (
	cache              Cache = NopCache{}
	cacheNop           bool
	logger             *slog.Logger
	postprocessDocChan chan *ExtractedDocument
	srv                http.Server
	tesConfig          TesConfig
	httpClient         *http.Client
)

func main() {
	tesConfig = NewTesConfigFromEnv()
	logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: tesConfig.logLevel}))
	// set static/global config of submodules
	tesswrap.Languages = tesConfig.TesseractLangs
	dehyphenator.RemoveNewlines = tesConfig.RemoveNewlines
	// Load PDF lib
	if err := LoadPdfLib(tesConfig.PdfLibName, tesConfig.PdfLibPath); err != nil {
		panic(err)
	}
	if pdfImpl.delete {
		// Delete the extracted file before process is terminated.
		// We could also delete it earlier, after it has been loaded but then a forked process couldn't use the same file.
		go func() {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, os.Interrupt)
			<-sigint
			deleteExtractedLib()
		}()
		defer deleteExtractedLib()
	}
	// one shot mode: don't start a server, just process a single file provided on the command line
	if len(os.Args) > 1 {
		// debug.SetGCPercent(-1)
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
		LogAndFixConfigIssues()
		PrintMetadataAndTextToStdout(os.Args[1])
		return
	}
	if tesConfig.Debug {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		// this might expose passwords in the log...
	}
	logger.Debug("Starting Text Extraction Service with config", "conf", tesConfig)
	LogAndFixConfigIssues()
	postprocessDocChan = make(chan *ExtractedDocument, 100)
	go saveAndCloseExtracedDocs()

	router := gin.New()
	router.Use(sloggin.New(logger), gin.Recovery())
	router.POST("/", ExtractBody)
	router.GET("/", ExtractRemote)
	router.HEAD("/", ExtractRemote)
	router.GET("/debug/vars", expvar.Handler())

	srv.Addr = tesConfig.SrvAddr
	srv.Handler = router

	nc, js := SetupNatsConnection(tesConfig)
	if nc == nil {
		cacheNop = true
	} else {
		RegisterNatsService(nc)
		defer nc.Drain()
		cache = InitCache(js, tesConfig)
	}

	if tesConfig.NoHttp {
		if nc == nil {
			logger.Error("Fatal: NATS not connected and HTTP disabled.")
			os.Exit(1)
		}
		wait := make(chan bool, 1)
		logger.Info("Service started with no HTTP endpoints. Waiting for interrupt.")
		<-wait
	}

	httpClient = &http.Client{
		Transport: &http.Transport{DisableCompression: tesConfig.HttpClientDisableCompression},
	}
	logger.Info("Service started", "address", srv.Addr)
	defer logger.Info("HTTP Server stopped.")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("Webserver failed", "err", err)
	}
}
