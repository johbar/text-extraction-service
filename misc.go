package main

import (
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/go-json-experiment/json"
	"github.com/johbar/text-extraction-service/v2/internal/pdfproc"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func parseForOcrOnce(d Document, ctx *model.Context, origin string) (*model.Context, error) {
	if ctx == nil {
		if len(d.Path()) == 0 {
			logger.Debug("Parsing data with pdfcpu for image extraction", "origin", origin)
			return pdfproc.ParseForImageExtraction(*d.Data())
		} else {
			logger.Debug("Parsing file with pdfcpu for image extraction", "origin", origin)
			return pdfproc.ParsePathForImageExtraction(d.Path())
		}
	}
	return ctx, nil
}

func WriteTextOrRunOcr(d Document, w io.Writer, origin string) error {
	var ctx *model.Context
	var err error
	if d.Pages() < 1 {
		return d.StreamText(w)
	}
	for i := range d.Pages() {
		text, hasImages := d.Text(i)
		if len(text) == 0 && hasImages && tesswrap.Initialized {
			ctx, err = parseForOcrOnce(d, ctx, origin)
			if err != nil {
				logger.Error("pdfcpu failed", "err", err, "origin", origin)
				continue
			}
			images, err := pdfproc.GetImages(ctx, i)
			if err != nil {
				logger.Error("Extracting images failed", "err", err, "origin", origin)
				continue
			}
			if len(images) < 1 {
				logger.Warn("No image found.", "origin", origin, "page", i)
			}
			for _, img := range images {
				logger.Info("Image found. Starting OCR", "origin", origin, "page", i, "type", img.FileType, "name", img.Name)
				ocrText, err := tesswrap.ImageReaderToText(img)
				if err != nil {
					logger.Error("Tesseract failed", "err", err, "origin", origin, "page", i, "imgName", img.Name)
					// we don't return that error, because we don't want to abort/fail the processing
					continue
				}
				if _, err := w.Write([]byte(ocrText)); err != nil {
					logger.Error("Could not write to output", "err", err)
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

// RunDehyphenator starts the dehyphenator process on another Go routine.
// It returns a channel that spawns a bool value, when all reading is done, and a pipewriter to write the input to
func RunDehyphenator(w io.Writer) (pw *io.PipeWriter) {
	pr, pw := io.Pipe()
	go func() {
		err := dehyphenator.DehyphenateReaderToWriter(pr, w)
		if err != nil {
			// If the dehyphenator failed, we proceed in streaming the content
			logger.Warn("Dehyphenator failed", "err", err)
			if _, err := io.Copy(w, pr); err != nil {
				logger.Error("RunDehyphenator: Could not write to output stream", "err", err)
			}
		}
		if err := pr.Close(); err != nil {
			logger.Error("RunDehyphenator: Could not close PipeReader in go routine")
		}
	}()
	return pw
}

// PrintMetadataAndTextToStdout prints a file's metadata (as JSON) on the first line, followed by the file's text content.
// The file can be local or remote (http/https). When url is "-", the file will be read from Stdin
func PrintMetadataAndTextToStdout(url string) {
	var doc Document
	var size int64 = -1
	var err error

	isHttp := strings.HasPrefix(url, "http")
	isStdIn := url == "-"
	if isHttp {
		resp, err := http.Get(url)
		if err != nil {
			logger.Error("HTTP error", "url", url, "err", err)
			os.Exit(1)
		}
		if resp.StatusCode >= 400 {
			logger.Error("HTTP error", "url", url, "status", resp.Status)
			os.Exit(1)
		}
		doc, err = NewDocFromStream(resp.Body, resp.ContentLength, url)
		resp.Body.Close()
		if err != nil {
			logger.Error("Could not process document", "url", url, "err", err)
			os.Exit(2)
		}
	} else {
		if isStdIn {
			doc, err = NewDocFromStream(os.Stdin, size, url)
		} else {
			doc, err = NewFromPath(url, url)
		}
		if err != nil {
			logger.Error("Could not process document", "url", url, "err", err)
			os.Exit(2)
		}
	}

	if err != nil {
		logger.Error("Could not process document", "url", url, "err", err)
		os.Exit(2)
	}

	err = json.MarshalWrite(os.Stdout, doc.MetadataMap())
	if err != nil {
		logger.Error("Could not print metadata", "err", err)
		os.Exit(1)
	}
	_, err = os.Stdout.WriteString("\n")
	if err != nil {
		logger.Error("Could not write to output", "err", err)
		os.Exit(1)
	}
	w := RunDehyphenator(os.Stdout)
	err = WriteTextOrRunOcr(doc, w, url)
	w.Close()
	doc.Close()
	if len(doc.Path()) > 1 && doc.Path() != url {
		err = os.Remove(doc.Path())
	}
	if err != nil {
		os.Exit(1)
	}
}

// LogAndFixConfigIssues logs warnings regarding configuration and fixes any issues of this kind
func LogAndFixConfigIssues() {
	buildinfo, _ := debug.ReadBuildInfo()

	logger.Debug("Info", "buildinfo", buildinfo)
	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Debug("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}

	if tesswrap.Initialized {
		if tessOk, whyNot := tesswrap.TesseractConfigOk(); !tessOk {
			logger.Warn("Language config is invalid. Tesseract will be disabled.", "reason", whyNot)
			tesswrap.Initialized = false
		}
	}

	if !docparser.Initialized {
		logger.Info("Neither wvWare, antiword nor catdoc found in PATH. We will not be able to extract legacy MS Word documents.")
	}

	logger.Info("PDF implementation", "lib", pdfImpl)
	// This ensures, that forked instances of TES will use the same lib
	os.Setenv("TES_PDF_LIB_NAME", pdfImpl.libShort)
	os.Setenv("TES_PDF_LIB_PATH", pdfImpl.LibPath)
}

func deleteExtractedLib() {
	pdflibwrappers.CloseLib()
	err := os.Remove(pdfImpl.LibPath)
	if err != nil {
		logger.Warn("Could not delete libpdfium in temp dir", "path", pdfImpl.LibPath)
		return
	}
	logger.Debug("libpdfium deleted in temp dir", "path", pdfImpl.LibPath)
}
