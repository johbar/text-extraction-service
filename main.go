package main

import (
	"errors"
	"os"
	"os/signal"

	"io"
	"log"
	"net/http"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
)

var closeDocChan chan Pdf
var srv http.Server

func init() {

}

func closeDocs(ch chan Pdf) {
	i := 1
	for doc := range ch {
		doc.Close()
		log.Printf("Document closed: %d", i)
		i++
	}
}

//ExtractBody returns the request body's plain text content.
//Returns a JSON encoded error message if the body is not a PDF.
func ExtractBody(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Println(err)
	}
	var doc Pdf
	contentType := mimetype.Detect(payload).Extension()
	if contentType == ".pdf" {
		doc, err = NewFromBytes(payload)
		if err != nil {
			log.Println(err)
			c.AbortWithStatus(http.StatusUnprocessableEntity)
			return
		}
		defer doc.Close()
		w := c.Writer
		w.WriteString(doc.Text())
	} else {
		log.Printf("Payload not a PDF, but " + contentType)
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity,
			gin.H{"error": "Not a PDF", "content_type": contentType})
		return
	}
}

func ExtractRemoteAsync(c *gin.Context) {
	url := validateUriParamUrl(c)
	if url == "" {
		return
	}
	defer log.Println("Enqueued:", url)
	processChan <- url
	c.JSON(http.StatusCreated, gin.H{"enqueued": true, "url": url})
}

func ExtractRemote(c *gin.Context) {
	url := validateUriParamUrl(c)
	if url == "" {
		return
	}
	defer log.Println("Done:", url)

	log.Println("Start parsing:", url)
	var doc Pdf
	doc, err := getRemoteDoc(url)
	if err != nil {
		log.Println(err)
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// addMetadataAsHeaders(c, doc)
	log.Println("Parsing done for:", url)
	defer func() {
		closeDocChan <- doc
		// log.Printf("Send to close channel: %s", url)
	}()

	doc.StreamText(c.Writer)

}

func ExtractAsJson(c *gin.Context) {
	url := validateUriParamUrl(c)
	if url == "" {
		return
	}
	doc, err := getRemoteDoc(url)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	}
	defer doc.Close()
	m := doc.Metadata()
	m["content"] = doc.Text()
	// jsonStr, err := json.MarshalIndent(m, "", "  ")
	status := http.StatusAccepted
	if err != nil {
		status = http.StatusUnprocessableEntity
	}

	c.JSON(status, m)
}

// func addMetadataAsHeaders(c *gin.Context, doc Pdf) {

// 	for k, v := range doc.Metadata() {
// 		k := "X-PDF-" + k
// 		c.Writer.Header().Add(k, v)
// 		// switch vStr := v.(type) {
// 		// case string:
// 			// c.Writer.Header().Add(k, v.(string))
// 		// case int:
// 			// c.Writer.Header().Add(k, strconv.Itoa(vStr))
// 		// }
// 	}

// }

func getRemoteDoc(url string) (doc Pdf, err error) {
	response, err := http.Get(url)
	if response.StatusCode >= 400 {
		log.Println("Error fetching ", url, err)
		response.Body.Close()
		return Pdf{}, errors.New(response.Status)
		// return Pdf{nil}, errors.New(response.Status)
	}
	if err != nil {
		log.Println("Error fetching ", url, err)
		// return Pdf{nil}, err
		return Pdf{}, err
	}
	defer response.Body.Close()
	doc, err = NewFromStream(response.Body)
	return
}

func main() {
	closeDocChan = make(chan Pdf, 100)
	go closeDocs(closeDocChan)
	router := gin.Default()
	router.POST("/pdf", ExtractBody)
	router.GET("/pdf", ExtractRemote)
	router.GET("/json", ExtractAsJson)
	router.GET("/pdf/forget", ExtractRemoteAsync)
	wg := startWorkers()
	host := ":8080"
	log.Println("Service started at", host)
	srv.Addr = ":8080"
	srv.Handler = router
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt)
	go handleInterrupt(shutdownChan, wg)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}
	log.Println("HTTP Server stopped.")
	wg.Wait()
	log.Println("All Go routines stopped.")
}
