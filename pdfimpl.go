package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"

	mupdf "github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/mupdf_purego"
	pdfium "github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/pdfium_purego"
	poppler "github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/poppler_purego"
)

var (
	// pdfImpl holds information about the PDF lib loaded by LoadLib.
	// Is nil when no lob has been loaded.
	pdfImpl pdfImplementation
)

type pdfImplementation struct {
	libShort       string
	LibDescription string
	LibPath        string
	delete         bool
}

// Document represents any kind of document this service can convert to plain text
type Document interface {
	// StreamText writes all text to w
	StreamText(w io.Writer) error
	// Pages returns the documents number of pages. Returns -1 if the concept is not applicable to the file type.
	Pages() int
	// Text returns a single page's text and true if there is at least one image on the page
	Text(int) (string, bool)
	// Data returns the underlying byte array or nil if the document was loaded from disk
	Data() *[]byte
	// Path returns the filesystem path a document was loaded from or an empty string if the was not loaded from disk
	// Path()
	// MetadataMap returns a map of Document properties, such as Author, Title etc.
	MetadataMap() map[string]string
	// Close releases resources associated with the document
	Close()
}

// LoadPdfLib returns a handle of one of the compatible PDF libs, signified by libName.
// It will be loaded from libPath. If libPath is an empty string, default locations and basenames will be attempted.
// Returns an error if the specified lib can not be loaded from one of theses paths.
func LoadPdfLib(libName string, libPath string) error {
	switch strings.ToLower(libName) {
	case "pdfium":
		libPath, err := pdfium.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "pdfium", LibDescription: "PDFium", LibPath: libPath}
		} else {
			var err2 error
			libPath, err2 = pdfium.ExtractLibpdfium()
			if err2 != nil {
				return errors.Join(err, err2)
			}
			logger.Debug("libpdfium extracted to temp dir", "path", libPath)
			libPath, err2 = pdfium.InitLib(libPath)
			if err2 == nil {
				pdfImpl = pdfImplementation{libShort: "pdfium", LibDescription: "PDFium", LibPath: libPath, delete: true}
				err = nil
			} else {
				err = errors.Join(err, err2)
			}
		}
		return err
	case "poppler":
		libPath, err := poppler.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "poppler", LibDescription: "Poppler (GLib) version " + poppler.Version(), LibPath: libPath}
		}
		return err
	case "mupdf":
		libPath, err := mupdf.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "mupdf", LibDescription: "MuPDF", LibPath: libPath}
		}
		return err
	}
	return errors.New("not a supported PDF library: " + libName)
}

// NewPdfFromBytes returns a PDF Document parsed by the particular PDF lib that was loaded before
func NewPdfFromBytes(data []byte, origin string) (doc Document, err error) {
	switch pdfImpl.libShort {
	case "pdfium":
		if pdfium.Lock.TryLock() {
			d, err := pdfium.Load(data)
			pdfium.Lock.Unlock()
			return d, err
		} else {
			r := bytes.NewReader(data)
			return NewDocFromForkedProcess(r, origin)
		}
	case "poppler":
		return poppler.Load(data)
	case "mupdf":
		return mupdf.Load(data)

	}
	// this should never happen as startup fails when no lib can be loaded:
	return nil, errors.New("no PDF implementation available")
}

// NewPdfFromPath returns a PDF Document parsed by the particular PDF lib that was loaded before
func NewPdfFromPath(path, origin string) (doc Document, err error) {
	switch pdfImpl.libShort {
	case "pdfium":
		if pdfium.Lock.TryLock() {
			d, err := pdfium.Open(path)
			pdfium.Lock.Unlock()
			return d, err
		} else {
			r, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			// FIXME where to close the os.File?
			return NewDocFromForkedProcess(r, origin)
		}
		// FIXME: add Poppler and MuPDF
	}
	return nil, errors.New("no PDF implementation available")

}
