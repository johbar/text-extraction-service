package main

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/officexmlparser"
	"github.com/johbar/text-extraction-service/v2/pkg/rtfparser"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

func NewDocFromStream(r io.Reader, origin string) (Document, error) {
	// FIXME make configurable
	r = io.LimitReader(r, 200*1024*1024)
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("zero-length data can not be parsed")
	}
	mtype := mimetype.Detect(data)
	logger.Debug("Detected", "mimetype", mtype.String(), "ext", mtype.Extension(), "origin", origin)
	if ext := strings.TrimPrefix(mtype.Extension(), "."); slices.Contains([]string{"odt", "odp", "docx", "pptx"}, ext) {
		return officexmlparser.NewFromBytes(data, ext)
	}

	switch mtype.Extension() {
	case ".pdf":
		return NewFromBytes(data, origin)
	case ".doc":
		if docparser.Initialized {
			return docparser.NewFromBytes(data)
		}
	case ".rtf":
		return rtfparser.NewFromBytes(data)
	}

	if tesswrap.Initialized && strings.HasPrefix(mtype.String(), "image/") {
		return NewDocFromImage(data, mtype.Extension()), nil
	}
	// returning a part of the content in case of errors helps with debugging webservers that return 2xx with an error message in the body
	return nil, fmt.Errorf("no suitable parser available for mimetype %s. content started with: %s", mtype.String(), string(data[:70]))
}
