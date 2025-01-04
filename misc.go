package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/johbar/text-extraction-service/v2/internal/pdfproc"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type ImageDoc struct {
	data     *[]byte
	mimetype string
}

func NewDocFromImage(data []byte, mimetype string) *ImageDoc {
	return &ImageDoc{data: &data, mimetype: mimetype}
}
func (d *ImageDoc) ProcessPages(w io.Writer, process func(pageText string, pageIndex int, w io.Writer, pdfData *[]byte)) {
	d.StreamText(w)
}

func (d *ImageDoc) StreamText(w io.Writer) {
	tesswrap.ImageReaderToTextWriter(bytes.NewReader(*d.data), w)
}

func (d *ImageDoc) Close() {
	// no op
}

func (d *ImageDoc) MetadataMap() map[string]string {
	meta := make(map[string]string)
	meta["x-doctype"] = d.mimetype
	// this isn't really useful and may even be expensive in terms of cpu/memory and new deps
	// so omitting it for now...

	// img, typ, err := image.Decode(bytes.NewReader(d.data))
	// if err != nil {
	// 	return
	// }

	// p := img.Bounds().Size()
	// meta["x-image-dimensions"] = fmt.Sprintf("%dx%d", p.X, p.Y)
	return meta
}

// WriteTextOrRunOcrOnPage writes pageText to w if it is not empty.
// Otherwise it looks for images on page pageNum and sends them to tesseract.
// The result is then being written to w.
func WriteTextOrRunOcrOnPage(pageText string, pageNum int, w io.Writer, pdfData *[]byte) {
	if len(strings.TrimSpace(pageText)) > 0 {
		// if len(pageText) > 0 {
		w.Write([]byte(pageText))
	} else if tesswrap.Initialized {
		logger.Info("No Text found. Looking for images for OCR", "page", pageNum)
		rs := bytes.NewReader(*pdfData)
		pdfproc.ProcessImages(rs, pageNum, func(img model.Image) {
			logger.Info("Image found. Starting OCR.", "page", pageNum, "name", img.Name, "type", img.FileType)
			err := tesswrap.ImageReaderToTextWriter(img, w)
			if err != nil {
				logger.Error("tesseract failed", "err", err)
			}
		})
	}
	// ensure there is a newline at the end of every page
	w.Write([]byte{'\n'})
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
			logger.Warn("dehyphenator failed", "err", err)
			io.Copy(w, pr)
		}
		pr.Close()
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
			os.Exit(1)
		}
		if resp.StatusCode >= 400 {
			logger.Error("HTTP error", "url", url, "err", err)
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
	doc, err := NewDocFromStream(stream)
	if err != nil {
		logger.Error("Could not process document", "url", url, "err", err)
		os.Exit(2)
	}
	meta, _ := json.Marshal(doc.MetadataMap())
	os.Stdout.Write(meta)
	os.Stdout.WriteString("\n")
	// w := os.Stdout
	done, w := RunDehyphenator(os.Stdout)
	doc.ProcessPages(w, WriteTextOrRunOcrOnPage)
	w.Close()
	<- done
}

// LogAndFixConfigIssues logs warnings regarding configuration and fixes any issues of this kind
func LogAndFixConfigIssues() {
	buildinfo, _ := debug.ReadBuildInfo()

	logger.Debug("Info", "buildinfo", buildinfo)
	if os.Getenv("GOMEMLIMIT") != "" {
		logger.Info("GOMEMLIMIT", "Bytes", debug.SetMemoryLimit(-1), "MBytes", debug.SetMemoryLimit(-1)/1024/1024)
	}

	if tesswrap.Initialized {
		if tessOk, whyNot := tesswrap.IsTesseractConfigOk(); !tessOk {
			logger.Warn("Language config is invalid. Tesseract will be disabled.", "reason", whyNot)
			tesswrap.Initialized = false
		}
	}

	if !docparser.Initialized {
		logger.Warn("wvWare is not available in PATH. We will not be able to extract legacy MS Word documents.")
	}

	logger.Info("PDF implementation", "lib", pdfImpl.Lib)
}
