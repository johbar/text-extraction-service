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

var (
	validate *validator.Validate
)

// ExtractedDocument contains pointers to metadata, textual content and URL of origin
type ExtractedDocument struct {
	Url      *string
	Metadata *map[string]string
	Text     []byte
	Doc      Document
}

type RequestParams struct {
	Url string `form:"url" binding:"required" json:"url" validate:"http_url"`
	//Ignore cached record
	NoCache bool `form:"noCache" json:"noCache"`
	//Send Metadata only, ignoring content
	Silent bool `form:"silent" json:"silent"`
}

func init() {
	validate = validator.New()
}

func saveAndCloseExtracedDocs() {
	for doc := range postprocessDocChan {
		doc.Doc.Close()
		logger.Debug("Document closed.", "url", doc.Url)
		for i := 0; i <= 5; i++ {
			err := cache.Save(doc)
			if err == nil {
				break
			}
			logger.Warn("Could not save text to cache", "retries", i, "url", doc.Url)
		}
	}
}

// ExtractBody returns the request body's plain text content.
// Returns a JSON encoded error message if the body is not parsable.
func ExtractBody(c *gin.Context) {
	origin := "POST request"
	doc, err := NewDocFromStream(c.Request.Body, &origin)
	if err != nil {
		logger.Error("Error parsing response body", "err", err)
		c.AbortWithStatus(http.StatusUnprocessableEntity)
		return
	}
	defer doc.Close()
	metadata := doc.MetadataMap()
	addMetadataAsHeaders(c.Writer.Header(), &metadata)
	done, pw := RunDehyphenator(c.Writer)
	doc.ProcessPages(pw, WriteTextOrRunOcrOnPage)
	pw.Close()
	<-done
}

func DocFromUrl(params RequestParams, w io.Writer, header http.Header) (status int, err error) {
	url := params.Url
	silent := params.Silent

	var noCache bool
	var metadata map[string]string

	if params.NoCache || cacheNop {
		noCache = true
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("Error when constructing GET request", "err", err, "url", url)
		return http.StatusInternalServerError, err
	}

	if !noCache {
		metadata = cache.GetMetadata(url)
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

	response, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Error fetching", "err", err, "url", url)
		return http.StatusNotFound, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		logger.Warn("Error fetching", "status", response.Status, "url", url)
		return response.StatusCode, fmt.Errorf("%s", response.Status)
	}
	if response.StatusCode == http.StatusNotModified {
		logger.Debug("URL has not been modified. Text will be served from cache", "url", url, "etag", response.Header.Get("etag"), "last-modified", response.Header.Get("last-modified"))
		addMetadataAsHeaders(header, &metadata)
		if silent {
			return http.StatusNotModified, nil
		}

		if err = cache.StreamText(url, w); err == nil {
			return http.StatusOK, nil
		}
		// We could not provide the client with cached text
		// Resume with parsing the file (again)
	}
	// We have no current version of the document but fetched it
	// so parse and extract it
	logger.Debug("Start parsing", "url", url, "content-length", response.ContentLength)
	var doc Document
	skipDehyphenator := false
	if tesConfig.ForkThreshold > -1 && response.ContentLength > tesConfig.ForkThreshold {
		// file size above threshold - fork a subprocess
		doc, err = NewDocFromForkedProcess(response.Body, &url)
		// the forked TES process does dehyphenation already
		// and the dehyphenator failes with input not containing newlines
		skipDehyphenator = true
	} else {
		doc, err = NewDocFromStream(response.Body, &url)
	}
	if err != nil {
		logger.Error("Error when parsing", "err", err, "url", url, "headers", response.Header)
		return http.StatusUnprocessableEntity, err
	}
	metadata = doc.MetadataMap()
	if etag := response.Header.Get("etag"); etag != "" {
		metadata["etag"] = etag
	}
	if lastmod := response.Header.Get("last-modified"); lastmod != "" {
		metadata["http-last-modified"] = lastmod
	}
	if contentLength := response.ContentLength; contentLength > 0 {
		metadata["http-content-length"] = fmt.Sprintf("%d", contentLength)
	}
	addMetadataAsHeaders(header, &metadata)
	logger.Debug("Parsing finished", "url", url)
	var text bytes.Buffer
	var mWriter io.Writer
	if silent {
		mWriter = io.MultiWriter(&text)
	} else {
		mWriter = io.MultiWriter(w, &text)
	}
	if skipDehyphenator {
		doc.ProcessPages(mWriter, WriteTextOrRunOcrOnPage)
	} else {
		done, pw := RunDehyphenator(mWriter)
		doc.ProcessPages(pw, WriteTextOrRunOcrOnPage)
		pw.Close()
		<-done
	}
	if !silent {
		logger.Debug("Streaming response done", "url", url)
	}
	extracted := &ExtractedDocument{
		Url:      &url,
		Text:     text.Bytes(),
		Metadata: &metadata,
		Doc:      doc,
	}
	postprocessDocChan <- extracted
	return http.StatusOK, nil
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

	if c.Request.Method == http.MethodHead {
		params.Silent = true
	}
	status, extractErr := DocFromUrl(params, c.Writer, c.Writer.Header())
	if extractErr != nil {
		c.AbortWithStatusJSON(status, gin.H{"code": status, "msg": extractErr.Error()})
		return
	}
	c.Status(status)
}

func addMetadataAsHeaders(header http.Header, metadata *map[string]string) {
	for k, v := range *metadata {
		header.Add(k, v)
	}
}
