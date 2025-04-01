package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/officexmlparser"
	"github.com/johbar/text-extraction-service/v2/pkg/rtfparser"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

type docFromWeb struct {
	Document
	delete bool
}

var (
	xmlBasedFormats = []string{".odt", ".odp", ".docx", ".pptx"}
	errZeroSize     = errors.New("zero-length data can not be parsed")
	errTooLarge     = errors.New("file too large")
)

func createTempFile(origin string) (*os.File, error) {
	dir := os.TempDir()
	var fileName string
	u, err := url.Parse(origin)
	if err != nil {
		fileName = "*-unknown"
	} else {
		fileName = "*-" + filepath.Base(u.Path)
	}
	f, err := os.CreateTemp(dir, fileName)
	return f, err
}

func saveToFs(r io.Reader, origin string) (string, error) {
	f, err := createTempFile(origin)
	if err != nil {
		return "", err
	}
	defer f.Close()
	logger.Debug("Saving file", "origin", origin, "path", f.Name())
	_, err = io.Copy(f, r)
	return f.Name(), err
}

func handleUnknownSize(r io.Reader, origin string) (Document, error) {
	// HTTP chunked encoding or reading from stdin
	buf := make([]byte, tesConfig.maxInMemoryBytes)
	logger.Debug("Reading stream with unknown size", "origin", origin, "buf", len(buf))
	bytesRead := 0
	n, err := io.ReadFull(r, buf)
	bytesRead += n
	isAll := err == io.EOF || err == io.ErrUnexpectedEOF
	isNotEvenAll := err == nil
	logger.Debug("Finished reading first chunk from stream with unknown size", "bytes", n, "err", err)
	if bytesRead >= int(tesConfig.maxInMemoryBytes) && (isAll || isNotEvenAll) {
		// file is too large for holding it in memory
		f, err := createTempFile(origin)
		if err != nil {
			return nil, err
		}
		logger.Info("Saving temporary file", "origin", origin, "path", f.Name())

		defer f.Close()
		if _, err := f.Write(buf); err != nil {
			return nil, err
		}
		if n, err := io.Copy(f, r); err != nil {
			return nil, err
		} else {
			logger.Debug("Finished reading remaining chunks from stream with unknown size", "bytes", n, "path", f.Name())
		}
		return NewFromPath(f.Name(), origin)
	} else if err != nil {
		// some other error occurred during reading the remote file or stdin
		return nil, err
	} else {
		// no error, file read was smaller than buf
		return NewFromData(buf, origin)
	}
}

func handleMediumSize(r io.Reader, size int64, origin string) (Document, error) {
	// file is too large to handle it in-memory
	path, err := saveToFs(r, origin)
	if err != nil {
		return nil, err
	}
	logger.Info("File saved", "path", path, "origin", origin, "size", humanize.Bytes(uint64(size)))
	return NewPdfFromPath(path, origin)
}

func handleSmallSize(r io.Reader, size int64, origin string) (Document, error) {
	data := make([]byte, size)
	_, err := io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errZeroSize
	}
	return NewFromData(data, origin)
}

func NewDocFromStream(r io.Reader, size int64, origin string) (Document, error) {
	if size > int64(tesConfig.maxFileSizeBytes) {
		// file is too large for downloading
		return nil, errTooLarge
	}
	if size == 0 {
		return nil, errZeroSize
	}
	if size < 0 {
		return handleUnknownSize(r, origin)
	}
	if size > int64(tesConfig.maxInMemoryBytes) {
		return handleMediumSize(r, size, origin)
	}
	// file is small enough to handle it in-memory
	return handleSmallSize(r, size, origin)
}

func NewFromData(data []byte, origin string) (Document, error) {
	mtype := mimetype.Detect(data)
	logger.Debug("Detected", "mimetype", mtype.String(), "ext", mtype.Extension(), "origin", origin)
	if ext := mtype.Extension(); slices.Contains(xmlBasedFormats, ext) {
		return officexmlparser.NewFromBytes(data, ext)
	}

	switch mtype.Extension() {
	case ".pdf":
		return NewPdfFromBytes(data, origin)
	case ".rtf":
		return rtfparser.NewFromBytes(data)
	}

	// there is no extension (like .doc) associated with these types
	if docparser.Initialized {
		switch mtype.String() {
		case "application/msword":
			fallthrough
		case "application/x-ole-storage":
			return docparser.NewFromBytes(data)
		}
	}
	if tesswrap.Initialized && strings.HasPrefix(mtype.String(), "image/") {
		return NewDocFromImage(data, mtype.Extension()), nil
	}
	// returning a part of the content in case of errors helps with debugging webservers that return 2xx with an error message in the body
	return nil, fmt.Errorf("no suitable parser available for mimetype %s. content started with: %s", mtype.String(), string(data[:70]))
}

func NewFromPath(path, origin string) (Document, error) {
	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, err
	}
	logger.Debug("Detected", "mimetype", mtype.String(), "ext", mtype.Extension(), "origin", origin)
	// FIXME
	if ext := mtype.Extension(); slices.Contains(xmlBasedFormats, ext) {
		return officexmlparser.Open(path, strings.TrimPrefix(ext, "."))
	}

	switch mtype.Extension() {
	case ".pdf":
		return NewPdfFromPath(path, origin)
	case ".rtf":
		return rtfparser.Open(path)
	}

	// there is no extension (like .doc) associated with these types
	if docparser.Initialized {
		switch mtype.String() {
		case "application/msword":
			fallthrough
		case "application/x-ole-storage":
			return docparser.Open(path)
		}
	}
	if tesswrap.Initialized && strings.HasPrefix(mtype.String(), "image/") {
		return OpenImage(path, mtype.Extension()), nil
	}
	// returning a part of the content in case of errors helps with debugging webservers that return 2xx with an error message in the body
	return nil, fmt.Errorf("no suitable parser available for mimetype %s, detected in %s from %s", mtype.String(), path, origin)
}
