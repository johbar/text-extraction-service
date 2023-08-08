package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type PdfMetadata map[string]string

var validate *validator.Validate

// ExtractedDocument contains pointers to PDF metadata, textual content and URL of origin
type ExtractedDocument struct {
	Url      *string
	Metadata *PdfMetadata
	Text     *bytes.Buffer
}

type RequestParams struct {
	Url     string `form:"url" binding:"required" validate:"http_url"`
	NoCache bool   `form:"noCache"`
	Silent  bool   `form:"silent"`
}

func init() {
	validate = validator.New()
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

func ExtractRemote(c *gin.Context) {
	var params RequestParams
	bindErr := c.BindQuery(&params)
	if bindErr != nil {
		c.AbortWithError(http.StatusBadRequest, bindErr)
		log.Printf("%v", c.Errors.JSON())
		return
	}
	valErr := validate.Struct(params)
	if valErr != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"err": fmt.Sprintf("%s is not a valid HTTP(S) URL", params.Url)})
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
			if etag, ok := metadata["etag"]; ok {
				req.Header.Add("If-None-Match", etag)
			}
			if lastMod, ok := metadata["http-last-modified"]; ok {
				req.Header.Add("If-Modified-Since", lastMod)
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
			addMetadataAsHeaders(c, &metadata)
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
	// doc, err := NewFromPipe(response.Body)
	doc, err := NewFromStream(response.Body)
	if err != nil {
		log.Printf("ExtractRemote: %v, %s", err, url)
		c.AbortWithError(http.StatusUnprocessableEntity, err)
		return
	}
	c.Status(http.StatusCreated)
	metadata = doc.Metadata()
	metadata["etag"] = response.Header.Get("etag")
	metadata["http-last-modified"] = response.Header.Get("last-modified")
	addMetadataAsHeaders(c, &metadata)
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

func addMetadataAsHeaders(c *gin.Context, metadata *PdfMetadata) {
	for k, v := range *metadata {
		c.Writer.Header().Add(k, v)
	}
}
