//go:build cache_nop

package main

import (
	"io"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type Cache interface {
	GetMetadata(url string) DocumentMetadata
	StreamText(url string, w io.Writer) error
	Save(doc *ExtractedDocument) error
}

type NopCache struct{}

func init() {
	cacheNop = true
}

func InitCache(js jetstream.JetStream, bucket string, replicas int) Cache {
	return NopCache{}
}

func (c NopCache) GetMetadata(url string) DocumentMetadata {
	return nil
}

func (c NopCache) StreamText(url string, w io.Writer) error {
	return nil
}

func (c NopCache) Save(doc *ExtractedDocument) error {
	return nil
}

// No-op NATS server connection
func SetupNatsConnection(conf TesConfig) (*nats.Conn, jetstream.JetStream) {
	return nil, nil
}
