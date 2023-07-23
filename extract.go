package main

import (
	"bytes"
	"io"
	"log"
	"net/http"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
)

type PdfMetadata map[string]string

// ExtractedDocument contains pointers to
type ExtractedDocument struct {
	Metadata *PdfMetadata
	Url      *string
	Text     *bytes.Buffer
}

type RequestParams struct {
	Url string `form:"url"`
	NoCache bool `form:"noCache"`
	Silent bool `form:"silent"`
}

func saveAndCloseExtracedDocs() {
	for {
		select {
		case doc := <-closeDocChan:
			doc.Close()
			log.Println("Document closed.")
		case data := <-saveExtractedDocChan:
			_, err := saveMetadataToCache(*data)
			if err != nil {
				log.Fatalf("ERROR: Failed to save metadata to cache: %v", err)
			}
			_, err2 := savePlaintextToCache(data)
			if err2 != nil {
				log.Fatalf("ERROR: Failed to save Text to cache: %v", err)
			}
			log.Printf("Saved to Cache: %s", *data.Url)
		}
	}
}

// ExtractBody returns the request body's plain text content.
// Returns a JSON encoded error message if the body is not a PDF.
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
	var params RequestParams
	bindErr := c.BindQuery(&params)
	if bindErr != nil {
		c.AbortWithError(http.StatusBadRequest, bindErr)
		log.Printf("%v", c.Errors.JSON())
		return
	}

	url := params.Url

	var (
		silent   bool
		noCache  bool
		metadata PdfMetadata
	)

	if params.Silent || c.Request.Method == "HEAD" {
		silent = true
	}

	if params.NoCache {
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
	log.Printf("Start parsing of %s. Length: %d", url, response.ContentLength)
	doc, err := NewFromPipe(response.Body)
	if err != nil {
		log.Printf("ExtractRemote: %v, %s", err, url)
		c.AbortWithError(http.StatusUnprocessableEntity, err)
		return
	}
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
	closeDocChan <- doc
	if noCache {
		return
	}
	metadata["etag"] = response.Header.Get("etag")
	metadata["http-last-modified"] = response.Header.Get("last-modified")
	extracted := &ExtractedDocument{
		Url:      &url,
		Text:     &text,
		Metadata: &metadata,
	}
	saveExtractedDocChan <- extracted
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
