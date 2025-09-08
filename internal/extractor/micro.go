package extractor

import (
	"bytes"
	"encoding/json/v2"
	"io"
	"net/http"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

func (e *Extractor) RegisterNatsService(nc *nats.Conn) {
	extractService, err := micro.AddService(nc, micro.Config{
		Name:        "extract-text",
		Version:     "1.0.0",
		Description: "Returns the plain text content of binary files like PDFs",
	})

	if err != nil {
		panic(err)
	}
	extractService.AddEndpoint("extract-remote",
		micro.HandlerFunc(e.handleUrl),
		micro.WithEndpointQueueGroup("text-extraction-service"))
	extractService.AddEndpoint("update-cache",
		micro.HandlerFunc(e.updateCache),
		micro.WithEndpointQueueGroup("text-extraction-service"))
}

// HandleUrl replies to a Nats request
func (e *Extractor) handleUrl(req micro.Request) {
	d := req.Data()
	var params RequestParams
	err := json.Unmarshal(d, &params)
	if err != nil {
		req.Error("invalid_params", err.Error(), nil)
		return
	}
	e.log.Info("Received Nats request", "params", params)
	var b bytes.Buffer
	header := http.Header{}
	_, err = e.DocFromUrl(params, &b, header)
	if err != nil {
		req.Error("failed", err.Error(), nil)
		return
	}
	req.Respond(b.Bytes(), micro.WithHeaders(micro.Headers(header)))
}

// UpdateCache responds with 'done' once a document has been added
// or refreshed in the cache
func (e *Extractor) updateCache(req micro.Request) {
	url := string(req.Data())
	params := RequestParams{Url: url, Silent: true}
	e.log.Info("Received Nats request", "params", params)
	header := http.Header{}
	_, err := e.DocFromUrl(params, io.Discard, header)
	if err != nil {
		req.Error("failed", err.Error(), nil)
		return
	}
	req.Respond([]byte("done"))
}
