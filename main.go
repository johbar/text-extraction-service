package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"log"
	"net/http"

	"github.com/gin-contrib/expvar"
	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
)

var (
	closeDocChan         chan Pdf
	saveExtractedDocChan chan *ExtractedDocument
	srv                  http.Server
	nc                   *nats.Conn
	plaintextBucket      nats.KeyValue
	metadataBucket       nats.KeyValue
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
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if os.Getenv("GOMEMLIMIT") != "" {
		log.Printf("GOMEMLIMIT=%v (%d MiB)", debug.SetMemoryLimit(-1), debug.SetMemoryLimit(-1)/1024/1024)
	}
	// buildinfo, _ := debug.ReadBuildInfo()
	// log.Printf("%v", buildinfo)
	closeDocChan = make(chan Pdf, 100)
	saveExtractedDocChan = make(chan *ExtractedDocument, 10)
	go saveAndCloseExtracedDocs()
	router := gin.Default()
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
				StoreDir: viper.GetString(confNatsDir),
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
		log.Printf("Connecting to Nats %s", connStr)
		nc, err = nats.Connect(connStr)
		if err != nil {
			panic(err)
		}
	}
	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("%e", err)
	}

	kvPlainTexts := &nats.KeyValueConfig{Bucket: "plaintexts_zstd", MaxValueSize: maxPayload, Storage: nats.FileStorage}
	kvMetaConf := &nats.KeyValueConfig{Bucket: "metadata", MaxValueSize: maxPayload, Storage: nats.FileStorage}
	plaintextBucket, err = js.CreateKeyValue(kvPlainTexts)
	if err != nil {
		log.Fatalf("%v", err)
	}
	metadataBucket, err = js.CreateKeyValue(kvMetaConf)
	log.Println("NATS server connected. ")
	if err != nil {
		log.Fatalf("%v", err)
	}
	log.Println("Service started at", srv.Addr)

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Printf("ERROR in webserver: %v", err)
	}
	log.Printf("Nats server still running...")
	log.Println("HTTP Server stopped.")

}
