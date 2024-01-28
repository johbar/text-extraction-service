package main

import (
	"bytes"
	"net/http"

	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go/micro"
)

func RegisterNatsService() {
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

}

//HandleUrl replies to a Nats request 
func HandleUrl(req micro.Request) {
	d := req.Data()
	var params RequestParams
	err := sonic.Unmarshal(d, &params)
	if err != nil {
		req.Error("invalid_params", err.Error(), nil)
		return
	}
	logger.Info("Received Nats request", "params", params)
	var b bytes.Buffer
	// var m DocumentMetadata
	header := http.Header{}
	_, err = DocFromUrl(params, &b, header)
	if err != nil {
		req.Error("failed", err.Error(), nil)
		return
	}
	req.Respond(b.Bytes(), micro.WithHeaders(micro.Headers(header)))
}
