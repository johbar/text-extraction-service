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

var xmlBasedFormats = []string{".odt", ".odp", ".docx", ".pptx"}

func createTempFilePath(origin string) (path string, err error) {
	dir, err := os.MkdirTemp("", "tes-download-*")
	if err != nil {
		return "", err
	}
	var fileName string
	u, err := url.Parse(origin)
	if err != nil {
		fileName = "unknown"
	} else {
		fileName = filepath.Base(u.Path)
	}
	return filepath.Join(dir, fileName), nil
}

func downloadFile(r io.Reader, origin string) (path string, err error) {
	filePath, err := createTempFilePath(origin)
	if err != nil {
		return "", err
	}
	f, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	logger.Debug("downloading file", "origin", origin, "path", filePath)
	_, err = io.Copy(f, r)
	return filePath, err
}

func NewDocFromStream(r io.Reader, size int64, origin string) (Document, error) {
	if size < 0 {
		// chunked encoding or reading from stdin
		logger.Debug("reading stream with unknown size")
		buf := make([]byte, 0, tesConfig.maxInMemoryBytes)
		n, err := r.Read(buf)
		isAll := err == io.EOF
		isNotEvenAll := err == nil
		logger.Debug("finished reading first chunk from stream with unknown size", "bytes", n, "err", err)
		if n == int(tesConfig.maxInMemoryBytes) && (isAll || isNotEvenAll) {
			// file is too large for holding it in memory
			p, err := createTempFilePath(origin)
			if err != nil {
				return nil, err
			}
			f, err := os.Open(p)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			if _, err := f.Write(buf); err != nil {
				return nil, err
			}
			if n, err := io.Copy(f, r); err != nil {
				return nil, err
			} else {
				logger.Debug("finished reading remaining chunks from stream with unknown size", "bytes", n, "path", p)
			}
			return NewFromPath(p, origin)
		} else if err != nil {
			// some other error occurred during reading the remote file or stdin
			return nil, err
		} else {
			// no error, file read was smaller than buf
			return newFromData(buf, origin)
		}
	}
	if size > int64(tesConfig.maxInMemoryBytes) {
		path, err := downloadFile(r, origin)
		if err != nil {
			return nil, err
		}
		logger.Info("file downloaded", "path", path, "origin", origin, "size", humanize.Bytes(uint64(size)))
		return NewPdfFromPath(path, origin)
	}
	data := make([]byte, 0, size)
	_, err := io.ReadFull(r, data)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("zero-length data can not be parsed")
	}
	return newFromData(data, origin)
}

func newFromData(data []byte, origin string) (Document, error) {

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
	// if ext := strings.TrimPrefix(mtype.Extension(), "."); xmlBasedFormats, ext) {
	// 	return officexmlparser.NewFromBytes(data, ext)
	// }

	switch mtype.Extension() {
	case ".pdf":
		return NewPdfFromPath(path, origin)
		// case ".rtf":
		// 	return rtfparser.NewFromBytes(data)
	}

	// there is no extension (like .doc) associated with these types
	// if docparser.Initialized {
	// 	switch mtype.String() {
	// 	case "application/msword":
	// 		fallthrough
	// 	case "application/x-ole-storage":
	// 		return docparser.NewFromBytes(data)
	// 	}
	// }
	// if tesswrap.Initialized && strings.HasPrefix(mtype.String(), "image/") {
	// 	return NewDocFromImage(data, mtype.Extension()), nil
	// }
	// returning a part of the content in case of errors helps with debugging webservers that return 2xx with an error message in the body
	return nil, fmt.Errorf("no suitable parser available for mimetype %s, detected in %s from %s", path, origin)
}
