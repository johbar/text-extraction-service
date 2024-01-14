package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type DocumentMetadata map[string]string

var validate *validator.Validate

// ExtractedDocument contains pointers to metadata, textual content and URL of origin
type ExtractedDocument struct {
	Url      *string
	Metadata *map[string]string
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
			logger.Debug("Document closed.")
		case doc := <-saveExtractedDocChan:
			err := saveToCache(doc)
			if err == nil {
				logger.Info("Saved to Cache", "url", *doc.Url)
			}
		}
	}
}

// ExtractBody returns the request body's plain text content.
// Returns a JSON encoded error message if the body is not parsable.
func ExtractBody(c *gin.Context) {
	doc, err := NewDocFromStream(c.Request.Body)
	if err != nil {
		logger.Error("Error parsing response body", "err", err)
		c.AbortWithStatus(http.StatusUnprocessableEntity)
		return
	}
	defer doc.Close()
	metadata := doc.MetadataMap()
	addMetadataAsHeaders(c, &metadata)
	doc.StreamText(c.Writer)
}

func ExtractRemote(c *gin.Context) {
	var params RequestParams
	bindErr := c.BindQuery(&params)
	if bindErr != nil {
		c.AbortWithError(http.StatusBadRequest, bindErr)
		logger.Warn("Invalid request", "err", c.Errors.JSON())
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
		metadata map[string]string
	)

	if params.Silent || c.Request.Method == http.MethodHead {
		silent = true
	}

	if params.NoCache || cacheNop {
		noCache = true
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("Error when constructing GET request", "err", err, "url", url)
	}

	if !noCache {
		metadata = getMetadataFromCache(url)
		if metadata != nil {
			if etag, ok := metadata["etag"]; ok {
				req.Header.Add("If-None-Match", etag)
			}
			if lastMod, ok := metadata["http-last-modified"]; ok {
				req.Header.Add("If-Modified-Since", lastMod)
			}
		}
	}
	logger.Debug("Issuing conditional GET request", "url", url, "headers", req.Header)
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Error("Error fetching", "err", err, "url", url)
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": err})
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		logger.Error("Error fetching", "err", err, "url", url)
		c.AbortWithStatusJSON(response.StatusCode, gin.H{
			"code":    response.StatusCode,
			"message": response.Status})
		return
	}
	if response.StatusCode == http.StatusNotModified {
		logger.Info("URL has not been modified. Text will be served from cache", "url", url, "etag", response.Header.Get("etag"), "last-modified", response.Header.Get("last-modified"))
		addMetadataAsHeaders(c, &metadata)
		if silent {
			c.Status(http.StatusNotModified)
			return
		}
		streamPlaintext(url, c.Writer)
		return
	}
	// We have no current version of the document but fetched it
	// so parse and extract it
	logger.Debug("Start parsing", "url", url, "content-length", response.ContentLength)
	doc, err := NewDocFromStream(response.Body)
	if err != nil {
		logger.Error("Error when parsing", "err", err, "url", url)
		c.AbortWithError(http.StatusUnprocessableEntity, err)
		return
	}
	c.Status(http.StatusCreated)
	metadata = doc.MetadataMap()
	metadata["etag"] = response.Header.Get("etag")
	metadata["http-last-modified"] = response.Header.Get("last-modified")
	addMetadataAsHeaders(c, &metadata)
	logger.Debug("Finished parsing", "url", url)
	var text bytes.Buffer
	var mWriter io.Writer
	if silent {
		mWriter = io.MultiWriter(&text)
	} else {
		mWriter = io.MultiWriter(c.Writer, &text)
	}
	doc.StreamText(mWriter)
	if !silent {
		logger.Debug("Streaming response done", "url", url)
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

func addMetadataAsHeaders(c *gin.Context, metadata *map[string]string) {
	for k, v := range *metadata {
		c.Writer.Header().Add(k, v)
	}
}
