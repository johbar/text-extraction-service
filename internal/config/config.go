package config

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/dustin/go-humanize"
	"go-simpler.org/env"
)

var Log = slog.Default()

// TesConfig represents the configuration of this service
type TesConfig struct {
	// Name of the object store or key-value bucket in NATS to use
	// Default: TES_PLAINTEXTS
	Bucket string `env:"TES_BUCKET" default:"TES_PLAINTEXTS"`
	// wether to expose embedded NATS server to other clients. Default: false
	ExposeNats bool `env:"TES_EXPOSE_NATS" default:"false"`
	// Add source info to log statement. Default: false
	Debug bool `env:"TES_DEBUG" default:"false"`
	// If true the service will exit with an error if NATS or JetStream can't be connected
	FailWithoutJetstream bool `env:"TES_FAIL_WITHOUT_JS" default:"true"`
	// Maximum content length (size in bytes) of a file that is being converted in-process
	// rather by a subprocess in fork-exec style. Default: 2 MiB
	ForkThreshold int64 `env:"TES_FORK_THRESHOLD" default:"2097152"`
	// Disable Accept-Encoding=gzip header in outgoing HTTP Requests
	HttpClientDisableCompression bool `env:"TES_HTTP_CLIENT_DISABLE_COMPRESSION" default:"false"`
	// Log level (DEBUG, INFO, WARN, ERROR)
	LogLevelStr string `env:"TES_LOG_LEVEL" default:"INFO"`
	LogLevel    slog.Level
	// Maximum size a file may have; processing is aborted if a requested file is bigger
	MaxFileSize      string `env:"TES_MAX_FILE_SIZE" default:"300Mib"`
	MaxFileSizeBytes uint64
	// maximum size of a file fetched from a web server to be processed solely in-memory instead of being downloaded
	MaxInMemory      string `env:"TES_MAX_IN_MEMORY" default:"2MiB"`
	MaxInMemoryBytes uint64
	// NATS max msg size (embedded server only)
	NatsMaxPayload int32 `env:"TES_MAX_PAYLOAD" default:"8388608"`
	// embedded NATS server storage location. Default: /tmp/nats
	NatsStoreDir string `env:"TES_NATS_STORE_DIR"`
	// embedded NATS server host/ip address, if exposed. Default: localhost
	NatsHost string `env:"TES_NATS_HOST" default:"localhost"`
	// embedded NATS server port, if exposed. Default: 4222
	NatsPort int `env:"TES_NATS_PORT" default:"4222"`
	// External NATS URL, e.g. nats://localhost:4222
	NatsUrl string `env:"TES_NATS_URL"`
	// Timeout for the external NATS connection
	NatsTimeout time.Duration `env:"TES_NATS_TIMEOUT" default:"15s"`
	// NatsConnectRetries is the number of attempts to connect to external NATS server(s)
	NatsConnectRetries int `env:"TES_NATS_CONNECT_RETRIES" default:"10"`
	// if true, disable HTTP Server in favor of NATS Microservice interface
	NoHttp bool `env:"TES_NO_HTTP" default:"false"`
	// name of the PDF implementation to load; either "pdfium", "poppler" or "mupdf"
	PdfLibName string `env:"TES_PDF_LIB_NAME" default:"pdfium"`
	// Path of the shared object file; can be empty (to use defaults) or just the basename (e.g. "libmupdf.so")
	PdfLibPath string `env:"TES_PDF_LIB_PATH"`
	// if true, extracted text will be compacted by replacing newlines with whitespace
	RemoveNewlines bool `env:"TES_REMOVE_NEWLINES" default:"true"`
	// How many replicas of the bucket to create. Default: 1
	Replicas int `env:"TES_REPLICAS" default:"1"`
	// HTTP listen address and/or port. Default: ':8080'
	SrvAddr string `env:"TES_HOST_PORT" default:":8080"`
	// List of 3-letter language codes, separated by `+` to be passed to Tesseract
	// when doing OCR. Default: eng. NOTE: The languages need to be installed.
	TesseractLangs string `env:"TES_TESSERACT_LANGS" default:"Latin"`
}

// NewTesConfigFromEnv returns a service config object
// populated with defaults and values from environment vars
func NewTesConfigFromEnv() (*TesConfig, error) {
	var cfg TesConfig
	if err := env.Load(&cfg, nil); err != nil {
		return nil, err
	}
	err := cfg.LogLevel.UnmarshalText([]byte(cfg.LogLevelStr))
	if err != nil {
		return nil, fmt.Errorf("parsing log level from env: %w", err)
	}
	maxbytes, err := humanize.ParseBytes(cfg.MaxInMemory)
	if err != nil {
		return nil, fmt.Errorf("parsing max in memory file size from env: %w", err)
	}
	cfg.MaxInMemoryBytes = maxbytes
	maxSize, err := humanize.ParseBytes(cfg.MaxFileSize)
	if err != nil {
		return nil, fmt.Errorf("parsing max file size from env: %w", err)
	}
	cfg.MaxFileSizeBytes = maxSize
	return &cfg, nil
}
