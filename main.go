package main

import (
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/gin-contrib/expvar"
	"github.com/gin-gonic/gin"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	sloggin "github.com/samber/slog-gin"
)

var (
	cache                Cache
	cacheNop             bool
	closeDocChan         chan Document
	pdfImplementation    string // Which lib is being used for PDFs?
	logger               *slog.Logger
	saveExtractedDocChan chan *ExtractedDocument
	srv                  http.Server
	tesConfig            TesConfig
	httpClient           *http.Client
)

func main() {
	tesConfig = NewTesConfigFromEnv()
	dehyphenator.RemoveNewlines = tesConfig.RemoveNewlines
	args := os.Args
	// one shot mode: don't start a server, just process a single file provided on the command line
	if len(args) > 1 {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
		PrintMetadataAndTextToStdout(args[1])
		return
	}
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	closeDocChan = make(chan Document, 100)
	saveExtractedDocChan = make(chan *ExtractedDocument, 100)
	go saveAndCloseExtracedDocs()

	router := gin.New()
	router.Use(sloggin.New(logger), gin.Recovery())
	router.POST("/", ExtractBody)
	router.GET("/", ExtractRemote)
	router.HEAD("/", ExtractRemote)
	router.GET("/debug/vars", expvar.Handler())

	srv.Addr = tesConfig.SrvAddr
	srv.Handler = router
	if tesConfig.Debug {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}

	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Info("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}
	buildinfo, _ := debug.ReadBuildInfo()
	logger.Debug("Info", "buildinfo", buildinfo)

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
	logger.Info("PDF implementation", "lib", pdfImplementation)
	if !docparser.Initialized {
		logger.Warn("wvWare is not available in PATH. We will not be able to extract legacy MS Word documents.")
	}
	httpClient = &http.Client{
		Transport: &http.Transport{DisableCompression:  tesConfig.HttpClientDisableCompression},
	}
	logger.Info("Service started", "address", srv.Addr)
	defer logger.Info("HTTP Server stopped.")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		logger.Error("Webserver failed", "err", err)
	}
}
