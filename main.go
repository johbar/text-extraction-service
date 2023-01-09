package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"

	// "errors"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gabriel-vasile/mimetype"
)

var closeDocChan chan PopplerPdf
var processChan chan string
var shutdownChan chan struct{}
var srv http.Server

func init() {
	closeDocChan = make(chan PopplerPdf, 100)
	go closeDocs(closeDocChan)
}

func closeDocs(ch chan PopplerPdf) {
	i := 1
	for doc := range ch {
		doc.Close()
		log.Printf("Document closed: %d", i)
		i++
	}
}

func Extract(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Println(err)
	}
	var doc PopplerPdf
	if mimetype.Detect(payload).Extension() == ".pdf" {

		doc, err = NewFromBytes(payload)
		if err != nil {
			log.Println(err)
			return
		}
		defer func() {
			closeDocChan <- doc
		}()

	}
	w := c.Writer
	w.WriteString(doc.Text())
}

func ExtractRemoteAsync(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	defer log.Println("Done:", url)
	if url == "" {
		w.WriteHeader(400)
		w.Write([]byte("Expected ?url=http..."))
		return
	}
	processChan <- url
	w.Write([]byte("Enqueued " + url + "\n"))
}

func ExtractRemote(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	defer log.Println("Done:", url)
	if url == "" {
		w.WriteHeader(400)
		w.Write([]byte("Expected ?url=http..."))
		return
	}

	log.Println("Start parsing:", url)
	var doc PopplerPdf
	doc, err := getRemoteDoc(url)
	if err != nil {
		log.Println(err)
		w.WriteHeader(422)
		w.Write([]byte(err.Error()))
		return
	}

	addMetadataAsHeaders(w, doc)
	log.Println("Parsing done for:", url)
	defer func() {
		closeDocChan <- doc
		// log.Printf("Send to close channel: %s", url)
	}()

	doc.StreamText(w)

}

func ExtractAsJson(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Expected ?url=http..."))
		return
	}
	doc, err := getRemoteDoc(url)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		log.Print(err)
		w.Write([]byte(err.Error()))
		return
	}
	defer doc.Close()
	m := *doc.Metadata()
	m["content"] = doc.Text()
	jsonStr, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	w.Write(jsonStr)
}

func addMetadataAsHeaders(w http.ResponseWriter, doc PopplerPdf) {
	if doc.HasMetadata() {
		for k, v := range *doc.Metadata() {
			k := "X-" + k
			switch vStr := v.(type) {
			case string:
				w.Header().Add(k, vStr)
			case int:
				w.Header().Add(k, strconv.Itoa(vStr))
			}
		}
	}
}

func getRemoteDoc(url string) (doc PopplerPdf, err error) {
	response, err := http.Get(url)
	if err != nil {
		return PopplerPdf{nil}, err
	}
	defer response.Body.Close()
	doc, err = NewFromStream(response.Body)
	return
}

func processEnqueuedUrls(ch <-chan string) {
	log.Print("Worker started.")
	for url := range ch {
		log.Println("Start parsing:", url)
		var doc PopplerPdf
		doc, err := getRemoteDoc(url)
		if err != nil {
			log.Println(err.Error(), url)
		}
		doc.Text()
		doc.Close()
		log.Println("Finished: " + url)
	}
	log.Println("Worker stopped.")
}

func waitForQueueEmpty(w http.ResponseWriter, r *http.Request) {
	for len(processChan) > 0 {
		left := fmt.Sprintf("Elements left in queue: %d\n", len(processChan))
		w.Write([]byte(left))
		// cast the ResponseWriter to Flusher in order to send every line to the client instantly
		if f, ok := w.(http.Flusher); ok { 
			f.Flush() 
			}
		time.Sleep(time.Second)
	}
	w.Write([]byte("Queue is empty.\n"))
}

func shutdown(w http.ResponseWriter, r *http.Request) {
	log.Println("Server stopping gracefully.")
	close(processChan)
	time.Sleep(time.Second)
	for len(processChan) > 0 {
		left := fmt.Sprintf("Elements left in queue: %d\n", len(processChan))
		w.Write([]byte(left))
		// cast the ResponseWriter to Flusher in order to send every line to the client instantly
		if f, ok := w.(http.Flusher); ok { 
			f.Flush() 
			}
		time.Sleep(time.Second)
	}
	w.Write([]byte("Queue is empty. Shutting down.\n"))
	go shutdownServer()
	shutdownChan <- struct{}{}
}

func shutdownServer() {
	<-shutdownChan
	srv.Shutdown(context.Background())
	shutdownChan <- struct{}{}
}

func main() {
	router := gin.Default()
	router.POST("/pdf", Extract)
	router.GET("/pdf", ExtractRemote)
	router.GET("/json", ExtractAsJson)
	router.GET("/pdf/forget", ExtractRemoteAsync)
	router.GET("/shutdown", shutdown)
	router.GET("/wait", waitForQueueEmpty)
	host := ":8080"
	processChan = make(chan string, 100_000_000)
	for i := 0; i < runtime.NumCPU(); i++ {
		go processEnqueuedUrls(processChan)
	}
	shutdownChan = make(chan struct{})
	log.Println("Service started at", host)
	srv.Addr = ":8080"
	srv.Handler = router
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	log.Println("Server stopping.")
	<-shutdownChan
	log.Println("Server stopped.")
}
