package main

import (
	"io"
)

type Cache interface {
	GetMetadata(url string) DocumentMetadata
	StreamText(url string, w io.Writer) error
	Save(doc *ExtractedDocument) error
}

type NopCache struct{}

func (c NopCache) GetMetadata(url string) DocumentMetadata {
	return nil
}

func (c NopCache) StreamText(url string, w io.Writer) error {
	return nil
}

func (c NopCache) Save(doc *ExtractedDocument) error {
	return nil
}
