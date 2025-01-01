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
	pdfImpl pdfImplementation
)

type pdfImplementation struct {
	libShort string
	Lib      string
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

func LoadLib(libName string, libPath string) error {
	switch strings.ToLower(libName) {
	case "poppler":
		err := poppler.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "poppler", Lib: "Poppler (GLib) version " + poppler.Version()}
		}
		return err
	case "pdfium":
		err := pdfium.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "pdfium", Lib: "PDFium"}
		}
		return err
	case "mupdf":
		err := mupdf.InitLib(libPath)
		if err == nil {
			pdfImpl = pdfImplementation{libShort: "mupdf", Lib: "MuPDF"}
		}
		return err
	}
	return errors.New("not a supported PDF library: " + libName)
}

func NewFromBytes(data []byte) (doc Document, err error) {
	switch pdfImpl.libShort {
	case "poppler":
		return poppler.Load(data)
	case "pdfium":
		if libIsFree := pdfium.Lock.TryLock(); libIsFree {
			pdfium.Lock.Unlock()
			pdf, err := pdfium.Load(data)
			return pdf, err
		} else {
			r := bytes.NewReader(data)
			return NewDocFromForkedProcess(r)
		}
	case "mupdf":
		return mupdf.Load(data)

	}
	return nil, errors.New("no pdf implementation available")
}
