package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
)

type DocumentMetadata = map[string]string

const lastModified string = "last-modified"

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
	Url string `form:"url" json:"url" validate:"http_url"`
	//Ignore cached record
	NoCache bool `form:"noCache" json:"noCache"`
	//Send Metadata only, ignoring content
	Silent bool `form:"silent" json:"silent"`
}

func init() {
	validate = validator.New()
}

func saveCloseAndDeleteExtracedDocs() {
	for doc := range postprocessDocChan {
		doc.Doc.Close()
		logger.Debug("Document closed.", "url", doc.Url)
		if len(doc.Doc.Path()) > 0 {
			// we can assume every file in this channel is a temporary file
			// created by ourself
			if err := os.Remove(doc.Doc.Path()); err != nil {
				logger.Error("could not remove temporary file", "err", err)
			} else {
				logger.Debug("temporary file removed", "path", doc.Doc.Path())
			}
		}
		if cacheNop {
			continue
		}
		for i := 0; i <= 5; i++ {
			info, err := cache.Save(doc)
			if err == nil {
				logger.Info("Saved text and metadata in NATS object store bucket", "url", *doc.Url, "chunks", info.Chunks, "size", info.Size)
				break
			}
			logger.Warn("Could not save text to cache", "retries", i, "url", doc.Url)
		}
	}
}

// ExtractBody returns the request body's plain text content.
// Returns a JSON encoded error message if the body is not parsable.
func ExtractBody(w http.ResponseWriter, r *http.Request) {
	origin := "POST request"
	doc, err := NewDocFromStream(r.Body, r.ContentLength, origin)
	if err != nil {
		logger.Error("Error parsing response body", "err", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(err.Error()))
		return
	}
	defer doc.Close()
	metadata := doc.MetadataMap()
	addMetadataAsHeaders(w.Header(), metadata)
	pw := RunDehyphenator(w)
	_ = WriteTextOrRunOcr(doc, pw, "<POST req>")
	pw.Close()
}

func DocFromUrl(params RequestParams, w io.Writer, header http.Header) (status int, err error) {
	url := params.Url
	silent := params.Silent

	var noCache bool

	if params.NoCache || cacheNop {
		noCache = true
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logger.Error("Error when constructing GET request", "err", err, "url", url)
		return http.StatusInternalServerError, err
	}

	metadata := addCacheValidationHeaders(noCache, req, url)
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
		logger.Debug("URL has not been modified. Text will be served from cache", "url", url, "etag", response.Header.Get("etag"), lastModified, response.Header.Get(lastModified))
		addMetadataAsHeaders(header, metadata)
		if silent {
			return http.StatusNotModified, nil
		}

		if err = cache.StreamText(url, w); err == nil {
			return http.StatusOK, nil
		} else {
			logger.Error("Could not receive text from NATS object store or write to output stream", "url", url, "err", err)
		}
		// We could not provide the client with cached text
		// Resume with parsing the file (again)
	}
	// We have no current version of the document but fetched it
	// so parse and extract it
	logger.Debug("Start parsing", "url", url, "content-length", response.ContentLength)
	doc, err, skipDehyphenator := constructDoc(url, response.Body, response.ContentLength)
	if err != nil {
		logger.Error("Parsing failed", "err", err, "url", url, "headers", response.Header)
		return http.StatusUnprocessableEntity, err
	}
	metadata = addHttpHeadersToMetadata(doc, response)
	addMetadataAsHeaders(header, metadata)
	logger.Debug("Finished parsing", "url", url)
	var text bytes.Buffer
	var mWriter io.Writer
	if silent {
		mWriter = io.MultiWriter(&text)
	} else {
		mWriter = io.MultiWriter(w, &text)
	}
	if skipDehyphenator {
		err = doc.StreamText(mWriter)
		if err != nil {
			logger.Error("Could not extract text from file or write to output stream", "url", url, "err", err)
			doc.Close()
			return 499, err
		}
	} else {
		pw := RunDehyphenator(mWriter)
		if err := WriteTextOrRunOcr(doc, pw, url); err != nil {
			pw.Close()
			doc.Close()
			// Client might have closed connection, so text couldn't be written
			// and is not complete. We don't want to save incomplete docs.
			return 499, err
		}
		pw.Close()
	}
	if !silent {
		logger.Debug("Streaming response done", "url", url)
	}
	extracted := ExtractedDocument{
		Url:      &url,
		Text:     text.Bytes(),
		Metadata: &metadata,
		Doc:      doc,
	}
	postprocessDocChan <- extracted
	return http.StatusOK, nil
}

func ExtractRemote(w http.ResponseWriter, r *http.Request) {
	var params RequestParams
	q := r.URL.Query()
	params.NoCache = q.Has("noCache") || q.Has("nocache")
	params.Silent = q.Has("silent")
	if r.Method == "HEAD" {
		params.Silent = true
	}
	params.Url = q.Get("url")
	valErr := validate.Struct(params)
	if valErr != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s is not a valid HTTP(S) URL", params.Url)
		return
	}

	status, extractErr := DocFromUrl(params, w, w.Header())
	if extractErr != nil {
		w.WriteHeader(status)
		fmt.Print(extractErr)
		return
	}
}

func addMetadataAsHeaders(header http.Header, metadata DocumentMetadata) {
	for k, v := range metadata {
		header.Add(k, v)
	}
}

func addCacheValidationHeaders(noCache bool, req *http.Request, url string) DocumentMetadata {
	if !noCache {
		metadata, err := cache.GetMetadata(url)
		if err != nil {
			logger.Error("Could not get metadata from NATS object store", "url", url, "err", err)
			return make(DocumentMetadata)
		}
		if etag, ok := metadata["etag"]; ok {
			req.Header.Add("If-None-Match", etag)
		}
		if lastMod, ok := metadata["http-last-modified"]; ok {
			req.Header.Add("If-Modified-Since", lastMod)
		}
		return metadata
	}
	return make(DocumentMetadata)
}

func constructDoc(url string, r io.Reader, contentLength int64) (d Document, err error, skipDehypenator bool) {
	if tesConfig.ForkThreshold > -1 && contentLength > tesConfig.ForkThreshold {
		// file size above threshold - fork a subprocess
		d, err = NewDocFromForkedProcess(r, url)
		// the forked TES process does dehyphenation already
		// and the dehyphenator fails with input not containing newlines
		skipDehypenator = true
	} else {
		d, err = NewDocFromStream(r, contentLength, url)
		skipDehypenator = false
	}
	return
}

func addHttpHeadersToMetadata(doc Document, response *http.Response) DocumentMetadata {
	metadata := doc.MetadataMap()
	if etag := response.Header.Get("etag"); etag != "" {
		metadata["etag"] = etag
	}
	if lastmod := response.Header.Get(lastModified); lastmod != "" {
		metadata["http-last-modified"] = lastmod
	}
	if contentLength := response.ContentLength; contentLength > 0 {
		metadata["http-content-length"] = fmt.Sprintf("%d", contentLength)
	}
	return metadata
}
