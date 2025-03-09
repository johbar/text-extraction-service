package main

import (
	"bytes"
	"io"
	"net/http"

	"github.com/go-json-experiment/json"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

func RegisterNatsService(nc *nats.Conn) {
	extractService, err := micro.AddService(nc, micro.Config{
		Name:        "extract-text",
		Version:     "1.0.0",
		Description: "Returns the plain text content of binary files like PDFs",
	})

	if err != nil {
		panic(err)
	}
	extractService.AddEndpoint("extract-remote",
		micro.HandlerFunc(HandleUrl),
		micro.WithEndpointQueueGroup("text-extraction-service"))
	extractService.AddEndpoint("update-cache",
		micro.HandlerFunc(UpdateCache),
		micro.WithEndpointQueueGroup("text-extraction-service"))
}

// HandleUrl replies to a Nats request
func HandleUrl(req micro.Request) {
	d := req.Data()
	var params RequestParams
	err := json.Unmarshal(d, &params)
	if err != nil {
		req.Error("invalid_params", err.Error(), nil)
		return
	}
	logger.Info("Received Nats request", "params", params)
	var b bytes.Buffer
	header := http.Header{}
	_, err = DocFromUrl(params, &b, header)
	if err != nil {
		req.Error("failed", err.Error(), nil)
		return
	}
	req.Respond(b.Bytes(), micro.WithHeaders(micro.Headers(header)))
}

// UpdateCache responds with 'done' once a document has been added
// or refreshed in the cache
func UpdateCache(req micro.Request) {
	url := string(req.Data())
	params := RequestParams{Url: url, Silent: true}
	logger.Info("Received Nats request", "params", params)
	header := http.Header{}
	_, err := DocFromUrl(params, io.Discard, header)
	if err != nil {
		req.Error("failed", err.Error(), nil)
		return
	}
	req.Respond([]byte("done"))
}
