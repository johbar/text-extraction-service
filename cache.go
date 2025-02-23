package main

import (
	"io"

	"github.com/nats-io/nats.go/jetstream"
)

type Cache interface {
	GetMetadata(url string) (DocumentMetadata, error)
	StreamText(url string, w io.Writer) error
	Save(doc *ExtractedDocument) (*jetstream.ObjectInfo, error)
}

type NopCache struct{}

func (c NopCache) GetMetadata(url string) (DocumentMetadata, error) {
	return nil, nil
}

func (c NopCache) StreamText(url string, w io.Writer) error {
	return nil
}

func (c NopCache) Save(doc *ExtractedDocument) (*jetstream.ObjectInfo, error) {
	return &jetstream.ObjectInfo{}, nil
}
