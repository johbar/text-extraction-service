package main

import (
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

func main() {
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
	viper.SetDefault("host", ":8080")
	viper.SetDefault("max_payload", 8*1024*1024)
	viper.AutomaticEnv()
	srv.Addr = viper.GetString("host")

	srv.Handler = router
	var maxPayload int32 = viper.GetInt32("max_payload")

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	ns, err := server.NewServer(
		&server.Options{
			// Host:               "localhost",
			JetStream:          true,
			MaxPayload:         maxPayload,
			JetStreamMaxMemory: 1024 * 1000,
			TLS:                false,
		})
	if err != nil {
		panic(err)
	}
	ns.ConfigureLogger()
	ns.Start()
	if !ns.ReadyForConnections(1 * time.Second) {
		panic("Nats not ready!")
	}

	nc, _ = nats.Connect(ns.ClientURL())
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
	ns.WaitForShutdown()
	log.Println("HTTP Server stopped.")

}
