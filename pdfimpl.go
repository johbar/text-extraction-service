package main

import (
	"bytes"
	"errors"
	"io"
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
	libShort      string
	LibDesciption string
	LibPath       string
}

// Document represents any kind of document this service can convert to plain text
type Document interface {
	// StreamText writes all text w
	StreamText(w io.Writer)
	// ProcessPages invokes process for every page of the document
	ProcessPages(w io.Writer, process func(pageText string, pageIndex int, w io.Writer, docData *[]byte))
	MetadataMap() map[string]string
	Close()
}

// LoadLib returns a handle of one of the compatible PDF libs, signified by libName.
// It will be loaded from libPath. If libPath is an empty string, default locations and basenames will be attempted.
// Returns an error if the specified lib can not be loaded from one of theses paths.
func LoadLib(libName string, libPath string) error {
	switch strings.ToLower(libName) {
	case "pdfium":
		libPath, err := pdfium.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "pdfium", LibDesciption: "PDFium", LibPath: libPath}
		} else if libInTempDir := pdfium.ExtractLibpdfium(); len(libInTempDir) > 0 {
			return LoadLib(libName, libInTempDir)
		}
		return err
	case "poppler":
		libPath, err := poppler.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "poppler", LibDesciption: "Poppler (GLib) version " + poppler.Version(), LibPath: libPath}
		}
		return err
	case "mupdf":
		libPath, err := mupdf.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "mupdf", LibDesciption: "MuPDF", LibPath: libPath}
		}
		return err
	}
	return errors.New("not a supported PDF library: " + libName)
}

// NewFromBytes returns a PDF Document parsed by the particular PDF lib that was loaded before
func NewFromBytes(data []byte) (doc Document, err error) {
	switch pdfImpl.libShort {
	case "pdfium":
		if libIsFree := pdfium.Lock.TryLock(); libIsFree {
			pdfium.Lock.Unlock()
			pdf, err := pdfium.Load(data)
			return pdf, err
		} else {
			r := bytes.NewReader(data)
			return NewDocFromForkedProcess(r)
		}
	case "poppler":
		return poppler.Load(data)
	case "mupdf":
		return mupdf.Load(data)

	}
	// this should never happen as startup fails when no lib can be loaded:
	return nil, errors.New("no pdf implementation available")
}