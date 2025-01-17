package main

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/rtfparser"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

func NewDocFromStream(r io.Reader, orgin *string) (Document, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("zero-length data can not be parsed")
	}
	mtype := mimetype.Detect(data)
	logger.Debug("Detected", "mimetype", mtype.String(), "orgin", orgin)
	switch mtype.String() {
	case "application/pdf":
		return NewFromBytes(data, orgin)
	case "application/msword":
		fallthrough
	case "application/x-ole-storage":
		if docparser.Initialized {
			return docparser.NewFromBytes(data)
		}
	case "text/rtf":
		return rtfparser.NewFromBytes(data)
	}
	if tesswrap.Initialized && strings.HasPrefix(mtype.String(), "image/") {
		return NewDocFromImage(data, mtype.String()), nil
	}
	// returning a part of the content helps with debugging webservers that return 2xx with an error message in the body
	return nil, fmt.Errorf("no suitable parser available for mimetype %s. content started with: %s", mtype.String(), string(data[:70]))
}
