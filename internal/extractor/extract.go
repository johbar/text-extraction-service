package extractor

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/johbar/text-extraction-service/v2/internal/cache"
	"github.com/johbar/text-extraction-service/v2/internal/config"
	"github.com/johbar/text-extraction-service/v2/internal/docfactory"
)

type RequestParams struct {
	Url string `form:"url" json:"url"`
	//Ignore cached record
	NoCache bool `form:"noCache" json:"noCache"`
	//Send Metadata only, ignoring content
	Silent bool `form:"silent" json:"silent"`
}

type Extractor struct {
	tesCache            cache.Cache
	df                  *docfactory.DocFactory
	log                 *slog.Logger
	httpClient          *http.Client
	cacheNop            bool
	postprocessDocsChan chan cache.ExtractedDocument
	tesConfig           *config.TesConfig
}

const lastModified string = "last-modified"

func New(config *config.TesConfig, df *docfactory.DocFactory, tesCache cache.Cache, logger *slog.Logger, httpClient *http.Client) *Extractor {
	postprocessDocsChan := make(chan cache.ExtractedDocument, 100)
	extract := &Extractor{
		tesCache:            tesCache,
		df:                  df,
		log:                 logger,
		postprocessDocsChan: postprocessDocsChan,
		tesConfig:           config,
		httpClient:          httpClient,
	}

	if httpClient == nil {
		extract.httpClient = http.DefaultClient
	}
	if logger == nil {
		extract.log = slog.New(slog.DiscardHandler)
	}
	_, extract.cacheNop = tesCache.(*cache.NopCache)
	go extract.saveCloseAndDeleteExtractedDocs()
	return extract
}

func (e *Extractor) saveCloseAndDeleteExtractedDocs() {
	for doc := range e.postprocessDocsChan {
		doc.Doc.Close()
		e.log.Debug("Document closed.", "url", doc.Url)
		if len(doc.Doc.Path()) > 0 {
			// we can assume every file in this channel is a temporary file
			// created by ourself
			if err := os.Remove(doc.Doc.Path()); err != nil {
				e.log.Error("could not remove temporary file", "err", err)
			} else {
				e.log.Debug("temporary file removed", "path", doc.Doc.Path())
			}
		}
		if e.cacheNop {
			continue
		}
		for i := 0; i <= 5; i++ {
			info, err := e.tesCache.Save(doc)
			if err == nil {
				e.log.Info("Saved text and metadata in NATS object store bucket", "url", *doc.Url, "chunks", info.Chunks, "size", info.Size)
				break
			}
			e.log.Warn("Could not save text to cache", "retries", i, "url", doc.Url)
		}
	}
}

// ExtractBody returns the request body's plain text content.
// Returns a JSON encoded error message if the body is not parsable.
func (e *Extractor) ExtractBody(w http.ResponseWriter, r *http.Request) {
	origin := "POST request"
	doc, err := e.df.NewDocFromStream(r.Body, r.ContentLength, origin)
	if err != nil {
		e.log.Error("Error parsing response body", "err", err)
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(err.Error()))
		return
	}
	defer doc.Close()
	metadata := doc.MetadataMap()
	addMetadataAsHeaders(w.Header(), metadata)
	pw, dehyphFinished := e.RunDehyphenator(w)
	_ = e.WriteTextOrRunOcr(doc, pw, "<POST req>")
	pw.Close()
	<-dehyphFinished
}

func (e *Extractor) fetch(url string, noCache bool) (*http.Response, cache.DocumentMetadata, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		e.log.Error("Error when constructing GET request", "err", err, "url", url)
		return nil, nil, err
	}

	metadata := e.addCacheValidationHeaders(noCache, req, url)
	e.log.Debug("Issuing conditional GET request", "url", url, "headers", req.Header)

	response, err := e.httpClient.Do(req)
	if err != nil {
		return response, metadata, fmt.Errorf("fetching %s: %w", url, err)
	}
	return response, metadata, err
}

func (e *Extractor) DocFromUrl(params RequestParams, w io.Writer, header http.Header) (status int, err error) {
	url := params.Url
	silent := params.Silent

	noCache := params.NoCache || e.cacheNop
	response, metadata, err := e.fetch(url, noCache)
	if err != nil {
		e.log.Error("Error fetching", "err", err, "url", url)
		return http.StatusBadRequest, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotModified {
		e.log.Debug("URL has not been modified. Text will be served from cache", "url", url, "etag", response.Header.Get("etag"), lastModified, response.Header.Get(lastModified))
		addMetadataAsHeaders(header, metadata)
		if silent {
			return http.StatusNotModified, nil
		}

		if err = e.tesCache.StreamText(url, w); err == nil {
			return http.StatusOK, nil
		} else {
			e.log.Error("Could not receive text from NATS object store or write to output stream", "url", url, "err", err)
		}
		// We could not provide the client with cached text
		// Resume with parsing the file (again)
	}
	// We have no current version of the document but fetched it
	// so parse and extract it
	e.log.Debug("Start parsing", "url", url, "content-length", response.ContentLength)
	doc, err, skipDehyphenator := e.constructDoc(url, response.Body, response.ContentLength)
	if err != nil {
		e.log.Error("Parsing failed", "err", err, "url", url, "headers", response.Header)
		return http.StatusUnprocessableEntity, err
	}
	metadata = addHttpHeadersToMetadata(doc, response)
	addMetadataAsHeaders(header, metadata)
	e.log.Debug("Finished parsing", "url", url)
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
			e.log.Error("Could not extract text from file or write to output stream", "url", url, "err", err)
			doc.Close()
			return 499, err
		}
	} else {
		pw, dehyphFinished := e.RunDehyphenator(mWriter)
		if err := e.WriteTextOrRunOcr(doc, pw, url); err != nil {
			pw.Close()
			doc.Close()
			// Client might have closed connection, so text couldn't be written
			// and is not complete. We don't want to save incomplete docs.
			return 499, err
		}
		pw.Close()
		<-dehyphFinished
	}

	if !silent {
		e.log.Debug("Streaming response done", "url", url)
	}
	extracted := cache.ExtractedDocument{
		Url:      &url,
		Text:     text.Bytes(),
		Metadata: &metadata,
		Doc:      doc,
	}
	e.postprocessDocsChan <- extracted
	return http.StatusOK, nil
}

func (e *Extractor) ExtractRemote(w http.ResponseWriter, r *http.Request) {
	var params RequestParams
	q := r.URL.Query()
	params.NoCache = q.Has("noCache") || q.Has("nocache")
	params.Silent = q.Has("silent")
	if r.Method == "HEAD" {
		params.Silent = true
	}
	url := q.Get("url")
	var errMsg string
	if !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		errMsg = fmt.Sprintf("not a valid HTTP(S) URL: %s", url)
		http.Error(w, errMsg, http.StatusBadRequest)
		return
	}
	params.Url =  url

	status, extractErr := e.DocFromUrl(params, w, w.Header())
	if extractErr != nil {
		e.log.Error("DocFromUrl failed", "status", status, "err", extractErr)
		w.WriteHeader(status)
		return
	}
}

func addMetadataAsHeaders(header http.Header, metadata cache.DocumentMetadata) {
	for k, v := range metadata {
		header.Add(k, v)
	}
}

func (e *Extractor) addCacheValidationHeaders(noCache bool, req *http.Request, url string) cache.DocumentMetadata {
	if !noCache {
		metadata, err := e.tesCache.GetMetadata(url)
		if err != nil {
			e.log.Error("Could not get metadata from NATS object store", "url", url, "err", err)
			return make(cache.DocumentMetadata)
		}
		if etag, ok := metadata["etag"]; ok {
			req.Header.Add("If-None-Match", etag)
		}
		if lastMod, ok := metadata["http-last-modified"]; ok {
			req.Header.Add("If-Modified-Since", lastMod)
		}
		return metadata
	}
	return make(cache.DocumentMetadata)
}

func (e *Extractor) constructDoc(url string, r io.Reader, contentLength int64) (d cache.Document, err error, skipDehypenator bool) {
	if e.tesConfig.ForkThreshold > -1 && contentLength > e.tesConfig.ForkThreshold {
		// file size above threshold - fork a subprocess
		d, err = e.df.NewDocFromForkedProcess(r, url)
		// the forked TES process does dehyphenation already
		// and the dehyphenator fails with input not containing newlines
		skipDehypenator = true
	} else {
		d, err = e.df.NewDocFromStream(r, contentLength, url)
		// our PDFium impl also forks a new process when the lib is in use already
		_, ok := d.(*docfactory.ForkedDoc)
		skipDehypenator = ok
	}
	return d, err, skipDehypenator
}

func addHttpHeadersToMetadata(doc cache.Document, response *http.Response) cache.DocumentMetadata {
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
