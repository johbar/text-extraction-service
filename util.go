package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"

	"github.com/gin-gonic/gin"
)

var processChan chan string

// isValidUrl tests a string to determine if it is a well-structured url or not.
func isValidUrl(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}

	u, err := url.Parse(toTest)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}

// validateUriParamUrl aborts the gin request with a JSON encoded error msg
// if the request contains no query param of name "url" with a a valid URL as value
func validateUriParamUrl(c *gin.Context) string {
	url := c.Query("url")
	msg := ""

	if !isValidUrl(url) {
		msg = fmt.Sprintf("%s is not a valid URL", url)
	}

	if url == "" {
		msg = "url query param missing"
	}

	if msg != "" {
		c.AbortWithStatusJSON(http.StatusBadRequest,
			gin.H{"error": msg})
		return ""
	}
	return url

}

func processEnqueuedUrls(ch <-chan string, wg *sync.WaitGroup, id int) {
	defer wg.Done()
	log.Printf("[%d] Worker started.", id)
	for url := range ch {
		log.Printf("[%d] Fetch and parse: %s", id, url)
		doc, err := getRemoteDoc(url)
		if err != nil {
			log.Println(err.Error(), url)
			continue
		}
		log.Printf("[%d] Parsed (%d Pages): %s", id, doc.GetNPages(), url)
		doc.Text()
		doc.Close()
		log.Printf("[%d] Finished: %s.", id, url)
	}
	log.Printf("[%d] Worker stopped.", id)
}

func startWorkers() (*sync.WaitGroup) {
	var wg sync.WaitGroup
	processChan = make(chan string, 100_000_000)
	for i := 0; i < runtime.NumCPU(); i++ {
		go processEnqueuedUrls(processChan, &wg, i)
		wg.Add(1)
	}
	return &wg
}

func handleInterrupt(ch <-chan os.Signal, wg *sync.WaitGroup) {

	sig := <-ch
	log.Printf("Received signal (%v). %d elements left in queue. Terminating gracefully...", sig, len(processChan))
	srv.Shutdown(context.Background())
	close(processChan)
	log.Printf("Channel closed. Waiting for workers to terminate...")
	wg.Wait()
	log.Printf("Workers terminated.")
}
