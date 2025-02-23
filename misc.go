package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/johbar/text-extraction-service/v2/internal/pdfproc"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func WriteTextOrRunOcr(d Document, w io.Writer, origin string) error {
	var ctx *model.Context
	var err error
	if d.Pages() < 1 {
		return d.StreamText(w)
	}
	for i := range d.Pages() {
		text, hasImages := d.Text(i)
		if len(text) == 0 && hasImages && tesswrap.Initialized {
			if ctx == nil {
				logger.Debug("Parsing file with pdfcpu for image extraction", "origin", origin)
				ctx, err = pdfproc.ParseForImageExtraction(*d.Data())
				if err != nil {
					logger.Error("pdfcpu failed", "err", err, "origin", origin)
					continue
				}
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
func RunDehyphenator(w io.Writer) (done chan bool, pw *io.PipeWriter) {
	finished := make(chan bool)
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
		finished <- true
		close(finished)
	}()
	return finished, pw
}

// PrintMetadataAndTextToStdout prints a file's metadata (as JSON) on the first line, followed by the file's text content.
// The file can be local or remote (http/https). When url is "-", the file will be read from Stdin
func PrintMetadataAndTextToStdout(url string) {
	var doc Document
	var stream io.ReadCloser
	if strings.HasPrefix(url, "http") {
		resp, err := http.Get(url)
		if err != nil {
			logger.Error("HTTP error", "url", url, "err", err)
			os.Exit(1)
		}
		if resp.StatusCode >= 400 {
			logger.Error("HTTP error", "url", url, "status", resp.Status)
			os.Exit(1)
		}
		stream = resp.Body
	} else {
		if url == "-" {
			stream = os.Stdin
		} else {
			f, err := os.Open(url)
			if err != nil {
				logger.Error("Could not open file", "err", err)
				os.Exit(1)
			}
			defer f.Close()
			stream = f
		}
	}
	doc, err := NewDocFromStream(stream, url)
	if err != nil {
		logger.Error("Could not process document", "url", url, "err", err)
		os.Exit(2)
	}
	meta, _ := json.Marshal(doc.MetadataMap())
	_, err = os.Stdout.Write(meta)
	if err != nil {
		logger.Error("Could not write to output", "err", err)
		os.Exit(1)
	}
	_, err = os.Stdout.WriteString("\n")
	if err != nil {
		logger.Error("Could not write to output", "err", err)
		os.Exit(1)
	}
	done, w := RunDehyphenator(os.Stdout)
	WriteTextOrRunOcr(doc, w, "<stdin>")
	w.Close()
	<-done
}

// LogAndFixConfigIssues logs warnings regarding configuration and fixes any issues of this kind
func LogAndFixConfigIssues() {
	buildinfo, _ := debug.ReadBuildInfo()

	logger.Debug("Info", "buildinfo", buildinfo)
	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Debug("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}

	if tesswrap.Initialized {
		if tessOk, whyNot := tesswrap.IsTesseractConfigOk(); !tessOk {
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
