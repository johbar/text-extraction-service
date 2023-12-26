package main

import (
	"errors"
	"io"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/rtfparser"
)

// Document represents any kind of document this service can convert to plain text
type Document interface {
	StreamText(io.Writer)
	MetadataMap() map[string]string
	Close()
}

func NewDocFromStream(r io.Reader) (Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	mtype := mimetype.Detect(data)
	logger.Info("Detected", "mimetype", mtype.String())
	switch mtype.String() {
	case "application/pdf":
		return NewFromBytes(data)
	case "application/msword":
		fallthrough
	case "application/x-ole-storage":
		return docparser.NewFromBytes(data)
	case "text/rtf":
		return rtfparser.NewFromBytes(data)
	}
	return nil, errors.New("new suitable parser available for mimetype" + mtype.String())
}
