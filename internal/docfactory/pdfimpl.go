package docfactory

import (
	"bytes"
	"errors"
	"os"
	"os/signal"
	"strings"

	"github.com/johbar/text-extraction-service/v4/internal/cache"
	"github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers"
	mupdf "github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers/mupdf_purego"
	pdfium "github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers/pdfium_purego"
	poppler "github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers/poppler_purego"
)

type pdfImplementation struct {
	LibShort       string
	LibDescription string
	LibPath        string
	delete         bool
}

// loadPdfLib returns a handle of one of the compatible PDF libs, signified by libName.
// It will be loaded from libPath. If libPath is an empty string, default locations and basenames will be tried.
// Returns an error if the specified lib can not be loaded from one of theses paths.
func (df *DocFactory) loadPdfLib(libName, libPath string) error {
	switch strings.ToLower(libName) {
	case "pdfium":
		imp, err := df.initPdfium(libName, libPath)
		if err == nil {
			df.pdfImpl = imp
		}
		return err

	case "poppler":
		libPath, err := poppler.InitLib(libPath)
		if err == nil {
			df.pdfImpl = pdfImplementation{LibShort: "poppler",
				LibDescription: "Poppler (GLib) version " + poppler.Version(),
				LibPath:        libPath,
			}
		}
		return err
	case "mupdf":
		libPath, err := mupdf.InitLib(libPath)
		if err == nil {
			df.pdfImpl = pdfImplementation{LibShort: "mupdf",
				LibDescription: "MuPDF",
				LibPath:        libPath,
			}
		}
		return err
	}
	return errors.New("not a supported PDF library: " + libName)
}

func (df *DocFactory) initPdfium(libName, libPath string) (pdfImplementation, error) {
	libPath, err := pdfium.InitLib(libPath)
	if err == nil {
		return pdfImplementation{
			LibShort:       "pdfium",
			LibDescription: "PDFium",
			LibPath:        libPath,
		}, nil
	}
	var err2 error
	libPath, err2 = pdfium.ExtractLibpdfium()
	if err2 != nil {
		return pdfImplementation{}, errors.Join(err, err2)
	}
	df.log.Debug("libpdfium extracted to temp dir", "path", libPath)
	libPath, err2 = pdfium.InitLib(libPath)
	if err2 == nil {
		// Delete the extracted file before process is terminated.
		// We could (at least on *nix OSes) also delete it earlier, after it has been loaded
		// but then a forked process couldn't use the same file.
		go func() {
			sigint := make(chan os.Signal, 1)
			signal.Notify(sigint, os.Interrupt)
			<-sigint
			pdflibwrappers.CloseLib()
			err := os.Remove(libPath)
			if err != nil {
				df.log.Warn("Could not delete libpdfium in temp dir", "path", libPath)
				return
			}
			df.log.Debug("libpdfium deleted in temp dir", "path", libPath)
		}()
		return pdfImplementation{LibShort: "pdfium", LibDescription: "PDFium", LibPath: libPath, delete: true}, nil
	} else {
		err = errors.Join(err, err2)
	}
	return pdfImplementation{}, err
}

// NewPdfFromBytes returns a PDF Document parsed by the particular PDF lib that was loaded before
func (df *DocFactory) NewPdfFromBytes(data []byte, origin string) (doc cache.Document, err error) {
	switch df.pdfImpl.LibShort {
	case "pdfium":
		if pdfium.Lock.TryLock() {
			d, err := pdfium.Load(data)
			pdfium.Lock.Unlock()
			return d, err
		} else {
			r := bytes.NewReader(data)
			return df.NewDocFromForkedProcess(r, origin)
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
func (df *DocFactory) NewPdfFromPath(path, origin string) (doc cache.Document, err error) {
	switch df.pdfImpl.LibShort {
	case "pdfium":
		if pdfium.Lock.TryLock() {
			d, err := pdfium.Open(path)
			pdfium.Lock.Unlock()
			return d, err
		} else {
			return df.NewDocFromForkedProcessPath(path, origin)
		}
	case "poppler":
		return poppler.Open(path)
	case "mupdf":
		return mupdf.Open(path)
	}
	return nil, errors.New("no PDF implementation available")
}

func (df *DocFactory) PdfImpl() pdfImplementation {
	return df.pdfImpl
}
