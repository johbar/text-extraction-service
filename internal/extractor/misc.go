package extractor

import (
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"encoding/json/v2"

	"github.com/johbar/text-extraction-service/v4/internal/cache"
	"github.com/johbar/text-extraction-service/v4/internal/pdfproc"

	"github.com/johbar/pdfcpu-lite/pkg/pdfcpu/model"
	"github.com/johbar/text-extraction-service/v4/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v4/pkg/tesswrap"
)

func (e *Extractor) parseForOcrOnce(d cache.Document, ctx *model.Context, origin string) (*model.Context, error) {
	if ctx != nil {
		return ctx, nil
	}
	e.log.Debug("Parsing PDF with pdfcpu for image extraction", "origin", origin)
	if len(d.Path()) == 0 {
		return pdfproc.ParseForImageExtraction(*d.Data())
	} else {
		return pdfproc.ParsePathForImageExtraction(d.Path())
	}
}

func (e *Extractor) WriteTextOrRunOcr(d cache.Document, w io.Writer, origin string) error {
	var ctx *model.Context
	var err error
	if d.Pages() < 1 {
		return d.StreamText(w)
	}
	for i := range d.Pages() {
		text, hasImages := d.Text(i)
		if len(text) < 200 && hasImages && tesswrap.Initialized {
			ctx, err = e.parseForOcrOnce(d, ctx, origin)
			if err != nil {
				e.log.Error("pdfcpu failed", "err", err, "origin", origin)
				continue
			}
			images, err := pdfproc.GetImages(ctx, i)
			if err != nil {
				e.log.Error("Extracting images failed", "err", err, "origin", origin)
				continue
			}
			if len(images) < 1 {
				e.log.Warn("No image found.", "origin", origin, "page", i)
			}
			for _, img := range images {
				e.log.Info("Image found. Starting OCR", "origin", origin, "page", i, "type", img.FileType, "name", img.Name)
				ocrText, err := tesswrap.ImageReaderToText(img)
				if err != nil {
					e.log.Error("Tesseract failed", "err", err, "origin", origin, "page", i, "imgName", img.Name)
					// we don't return that error, because we don't want to abort/fail the processing
					continue
				}
				if _, err := w.Write([]byte(ocrText)); err != nil {
					e.log.Error("writing OCRed text to output", "err", err)
					return err
				}
			}
		}
		if _, err := w.Write([]byte(text)); err != nil {
			return err
		}
		// ensure there is a newline at the end of every page
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// PrintMetadataAndTextToStdout prints a file's metadata (as JSON) on the first line, followed by the file's text content.
// The file can be local or remote (http/https). When url is "-", the file will be read from Stdin
func (e *Extractor) PrintMetadataAndTextToStdout(url string) {
	var doc cache.Document
	var size int64 = -1
	var err error

	isHttp := strings.HasPrefix(url, "http")
	isStdIn := url == "-"
	if isHttp {
		resp, err := http.Get(url)
		if err != nil {
			e.log.Error("HTTP error", "url", url, "err", err)
			os.Exit(1)
		}
		if resp.StatusCode >= 400 {
			e.log.Error("HTTP error", "url", url, "status", resp.Status)
			os.Exit(1)
		}
		doc, err = e.df.NewDocFromStream(resp.Body, resp.ContentLength, url)
		resp.Body.Close()
		if err != nil {
			e.log.Error("Could not process document", "url", url, "err", err)
			os.Exit(2)
		}
	} else {
		if isStdIn {
			doc, err = e.df.NewDocFromStream(os.Stdin, size, url)
		} else {
			doc, err = e.df.NewFromPath(url, url)
		}
		if err != nil {
			e.log.Error("Could not process document", "url", url, "err", err)
			os.Exit(2)
		}
	}

	if err != nil {
		e.log.Error("Could not process document", "url", url, "err", err)
		os.Exit(2)
	}

	err = json.MarshalWrite(os.Stdout, doc.MetadataMap())
	if err != nil {
		e.log.Error("Could not print metadata", "err", err)
		os.Exit(1)
	}
	_, err = os.Stdout.WriteString("\n")
	if err != nil {
		e.log.Error("Could not write to output", "err", err)
		os.Exit(1)
	}
	dw := dehyphenator.New(os.Stdout, e.tesConfig.RemoveNewlines)
	err = e.WriteTextOrRunOcr(doc, dw, url)
	dw.Close()
	doc.Close()
	if len(doc.Path()) > 1 && doc.Path() != url {
		err = os.Remove(doc.Path())
	}
	if err != nil {
		os.Exit(1)
	}
}

// FIXME: remove?
// LogAndFixConfigIssues logs warnings regarding configuration and fixes any issues of this kind
func (e *Extractor) LogAndFixConfigIssues() {
	buildinfo, _ := debug.ReadBuildInfo()

	e.log.Debug("Info", "buildinfo", buildinfo)
	if os.Getenv("GOMEMLIMIT") != "" {
		e.log.Debug("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}

	if tesswrap.Initialized {
		if tessOk, whyNot := tesswrap.TesseractConfigOk(); !tessOk {
			e.log.Warn("Language config is invalid. Tesseract will be disabled.", "reason", whyNot)
			tesswrap.Initialized = false
		}
	}

	e.log.Info("PDF implementation", "lib", e.df.PdfImpl())
	// This ensures, that forked instances of TES will use the same lib
	os.Setenv("TES_PDF_LIB_NAME", e.df.PdfImpl().LibShort)
	os.Setenv("TES_PDF_LIB_PATH", e.df.PdfImpl().LibPath)
}
