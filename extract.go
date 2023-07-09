package main

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
)

type Metadata map[string]string

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
	var (
		silent   bool
		noCache  bool
		metadata Metadata
	)

	if c.Query("silent") == "true" || c.Request.Method == "HEAD" {
		silent = true
	}

	if c.Query("nocache") == "true" {
		noCache = true
	}
	// log.Printf("Got metadata from Cache: %v", metadata)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("ExtractRemote: %v %s", err, url)
	}

	if !noCache {
		metadata = getMetaDataFromCache(url)
		if metadata != nil {
			if metadata["etag"] != "" {
				req.Header.Add("If-None-Match", metadata["etag"])
			}
			if metadata["http-last-modified"] != "" {
				req.Header.Add("If-Modified-Since", metadata["http-last-modified"])
			}
		}
	}
	log.Printf("Issuing GET request for %s with headers %v", url, req.Header)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("%v Error fetching %s", err, url)
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err})
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		log.Printf("%v Error fetching %s", err, url)
		c.AbortWithStatusJSON(response.StatusCode, gin.H{
			"code":    response.StatusCode,
			"message": response.Status})
		return
	}
	if response.StatusCode == http.StatusNotModified {
		log.Printf("URL %s has not been modified: etag=%s, last-modified=%s. Text will be served from cache", url, response.Header.Get("etag"), response.Header.Get("last-modified"))
		text := getPlaintextFromCache(url)
		if text != nil {
			log.Printf("Found plain text in cache: %s", url)
			addMetadataAsHeaders(c, metadata)
			if silent {
				c.Status(http.StatusNotModified)
				return
			}
			c.Writer.Write(text)
			return
		}
		log.Printf("Strange: Did not find plain text in cache: %s. Issue another GET request...", url)
		response, err = http.Get(url)
		if err != nil {
			log.Printf("%v Error fetching %s", err, url)
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err})
			return
		}
		defer response.Body.Close()
	}
	// We have no current version of the document but fetched it
	// so parse and extract it
	log.Println("Start parsing:", url)
	doc, err := NewFromStream(response.Body)
	if err != nil {
		log.Printf("ExtractRemote: %v, %s", err, url)
		c.AbortWithError(http.StatusUnprocessableEntity, err)
		return
	}
	defer doc.Close()
	c.Status(http.StatusCreated)
	metadata = doc.Metadata()
	addMetadataAsHeaders(c, metadata)
	log.Println("Parsing done for:", url)
	var text bytes.Buffer
	var mWriter io.Writer
	if silent {
		mWriter = io.MultiWriter(&text)
	} else {
		mWriter = io.MultiWriter(c.Writer, &text)
	}
	doc.StreamText(mWriter)
	if !silent {
		log.Printf("Streaming response done for %s", url)
	}
	// do this out-of-band with responding to the client
	if noCache {
		return
	}
	SaveToCache(response, text, metadata)
}

func SaveToCache(response *http.Response, text bytes.Buffer, metadata map[string]string) {
	url := response.Request.URL.String()
	savePlaintextToCache(url, text.Bytes())
	metadata["etag"] = response.Header.Get("etag")
	metadata["http-last-modified"] = response.Header.Get("last-modified")
	_, err := saveMetadataToCache(url, metadata)
	if err != nil {
		log.Printf("Error saving metadata to Cache: %v", err)
		return
	}
	log.Printf("Finished updating cache: %s", url)
}

// func ExtractAsJson(c *gin.Context) {
// 	url := validateUriParamUrl(c)
// 	if url == "" {
// 		return
// 	}
// 	doc, err := getRemoteDoc(url)
// 	if err != nil {
// 		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
// 	}
// 	defer doc.Close()
// 	m := doc.Metadata()
// 	m["content"] = doc.Text()
// 	// jsonStr, err := json.MarshalIndent(m, "", "  ")
// 	status := http.StatusAccepted
// 	if err != nil {
// 		status = http.StatusUnprocessableEntity
// 	}
// 	c.JSON(status, m)
// }

func addMetadataAsHeaders(c *gin.Context, metadata map[string]string) {
	for k, v := range metadata {
		c.Writer.Header().Add(k, v)
	}
}
