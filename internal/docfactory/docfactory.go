package docfactory

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/text-extraction-service/v4/internal/cache"
	"github.com/johbar/text-extraction-service/v4/internal/config"
	"github.com/johbar/text-extraction-service/v4/internal/imageparser"
	"github.com/johbar/text-extraction-service/v4/pkg/docparser"
	"github.com/johbar/text-extraction-service/v4/pkg/mmappool"
	"github.com/johbar/text-extraction-service/v4/pkg/officexmlparser"
	"github.com/johbar/text-extraction-service/v4/pkg/rtfparser"
	"github.com/johbar/text-extraction-service/v4/pkg/tesswrap"
)

var (
	xmlBasedFormats = []string{".odt", ".odp", ".docx", ".pptx"}
	errZeroSize     = errors.New("zero-length data can not be parsed")
	errTooLarge     = errors.New("file too large")
)

type DocFactory struct {
	MaxInMemoryBytes uint64
	MaxFileSizeBytes uint64
	pdfImpl          pdfImplementation
	log              *slog.Logger
	executable       string
	pool             *mmappool.Mempool
}

type PooledDoc struct {
	cache.Document
	poolBuffer []byte
	df         *DocFactory
}

func (d *PooledDoc) Close() {
	d.Document.Close()
	d.df.pool.Put(d.poolBuffer)
}

func (d *PooledDoc) HasNewlines() bool {
	return d.Document.HasNewlines()
}

func New(tesconfig *config.TesConfig, logger *slog.Logger) *DocFactory {
	exe, _ := os.Executable()
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	df := &DocFactory{
		MaxInMemoryBytes: tesconfig.MaxInMemoryBytes,
		MaxFileSizeBytes: tesconfig.MaxFileSizeBytes,
		log:              logger,
		executable:       exe,
		pool:             mmappool.New(int(tesconfig.MaxInMemoryBytes), 8, logger.With("mod", "docfactory mempool")),
	}

	err := df.loadPdfLib(tesconfig.PdfLibName, tesconfig.PdfLibPath)
	if err != nil {
		df.log.Error("PDF library could not be loaded", "err", err)
	}
	return df
}

func newTempFile(origin string) (*os.File, error) {
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

func (df *DocFactory) saveToFs(r io.Reader, origin string) (string, error) {
	f, err := newTempFile(origin)
	if err != nil {
		return "", fmt.Errorf("creating temp file for origin %s: %w", origin, err)
	}
	defer f.Close()
	df.log.Debug("Saving file", "origin", origin, "path", f.Name())
	_, err = io.Copy(f, r)
	return f.Name(), err
}

// handleUnknownSize deals with HTTP chunked encoding or reading from stdin
func (df *DocFactory) handleUnknownSize(r io.Reader, origin string) (cache.Document, error) {
	buf, _ := df.pool.Get()
	df.log.Debug("Reading stream of unknown size", "origin", origin, "buf", len(buf))
	bytesRead := 0
	n, err := io.ReadFull(r, buf[:df.MaxInMemoryBytes])
	bytesRead += n
	isAll := err == io.EOF || err == io.ErrUnexpectedEOF
	isNotEvenAll := err == nil
	df.log.Debug("Finished reading first chunk from stream of unknown size", "bytes", n, "err", err)
	if bytesRead >= int(df.MaxInMemoryBytes) && (isNotEvenAll) {
		// file is too large for holding it in memory
		f, err := newTempFile(origin)
		if err != nil {
			return nil, fmt.Errorf("creating tempfile for origin %s: %w", origin, err)
		}
		df.log.Info("Saving temporary file", "origin", origin, "path", f.Name())

		defer f.Close()
		if _, err := f.Write(buf[:df.MaxInMemoryBytes]); err != nil {
			return nil, err
		}
		if n, err := io.Copy(f, r); err != nil {
			return nil, err
		} else {
			df.log.Debug("Finished reading remaining chunks from stream of unknown size", "bytes", n, "path", f.Name())
		}
		df.pool.Put(buf)
		return df.NewFromPath(f.Name(), origin)
	}
	if isAll {
		// no error, file read was smaller than buf
		d, err := df.NewFromBytes(buf[:bytesRead], origin)
		return &PooledDoc{d, buf, df}, err
	}
	return nil, err
}

func (df *DocFactory) handleMediumSize(r io.Reader, size int64, origin string) (cache.Document, error) {
	// file is too large to handle it in-memory
	path, err := df.saveToFs(r, origin)
	if err != nil {
		return nil, err
	}
	df.log.Info("File saved", "path", path, "origin", origin, "size", humanize.Bytes(uint64(size)))
	return df.NewFromPath(path, origin)
}

func (df *DocFactory) handleSmallSize(r io.Reader, size int64, origin string) (cache.Document, error) {
	buf, err := df.pool.Get()
	if err != nil {
		return nil, err
	}

	n, err := io.ReadFull(r, buf[:size])
	if err != nil {
		return nil, err
	}
	d, err := df.NewFromBytes(buf[:n], origin)
	return &PooledDoc{d, buf, df}, err
}

func (df *DocFactory) NewDocFromStream(r io.Reader, size int64, origin string) (cache.Document, error) {
	if size > int64(df.MaxFileSizeBytes) {
		// file is too large for downloading
		return nil, errTooLarge
	}
	if size < 0 {
		return df.handleUnknownSize(r, origin)
	}
	if size == 0 {
		return nil, errZeroSize
	}
	if size > int64(df.MaxInMemoryBytes) {
		return df.handleMediumSize(r, size, origin)
	}
	// file is small enough to handle it in-memory
	return df.handleSmallSize(r, size, origin)
}

func (df *DocFactory) NewFromBytes(data []byte, origin string) (cache.Document, error) {
	mtype := mimetype.Detect(data)
	df.log.Debug("Detected", "mimetype", mtype.String(), "ext", mtype.Extension(), "origin", origin)
	if ext := mtype.Extension(); slices.Contains(xmlBasedFormats, ext) {
		return officexmlparser.NewFromBytes(data, ext)
	}

	switch mtype.Extension() {
	case ".pdf":
		return df.NewPdfFromBytes(data, origin)
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
		return imageparser.NewFromBytes(data, mtype.Extension()), nil
	}
	// returning a part of the content in case of errors helps with debugging webservers that return 2xx with an error message in the body
	return nil, fmt.Errorf("no suitable parser available for mimetype %s. content started with: %s", mtype.String(), string(data[:70]))
}

func (df *DocFactory) NewFromPath(path, origin string) (cache.Document, error) {
	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, err
	}
	df.log.Debug("Detected", "mimetype", mtype.String(), "ext", mtype.Extension(), "origin", origin)
	if ext := mtype.Extension(); slices.Contains(xmlBasedFormats, ext) {
		return officexmlparser.Open(path, strings.TrimPrefix(ext, "."))
	}

	switch mtype.Extension() {
	case ".pdf":
		return df.NewPdfFromPath(path, origin)
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
		return imageparser.Open(path, mtype.Extension()), nil
	}
	// returning a part of the content in case of errors helps with debugging webservers that return 2xx with an error message in the body
	return nil, fmt.Errorf("no suitable parser available for mimetype %s, detected in %s from %s", mtype.String(), path, origin)
}
