//go:build !mupdf

package main

import (
	"errors"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/go-poppler"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
)

type Pdf struct {
	*poppler.Document
}

func init() {
	slog.Info("Using Poppler (GLib) library", "version", poppler.Version())
}

func NewFromStream(stream io.ReadCloser) (doc *Pdf, err error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		logger.Error("NewFromStream: ", "error", err)
	}
	stream.Close()
	return NewFromBytes(data)
}

func NewFromBytes(data []byte) (doc *Pdf, err error) {
	if mimetype.Detect(data).Extension() != ".pdf" {
		return &Pdf{nil}, errors.New("not a PDF")
	}
	pDoc, err := poppler.Load(data)
	if err != nil {
		logger.Error("Could not load PDF", "error", err)
	}
	doc = &Pdf{pDoc}
	return
}

// Text returns the plain text content of the document
func (d *Pdf) Text() string {
	var buf strings.Builder
	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		buf.WriteString(page.Text())
		page.Close()
	}
	return buf.String()
}

// StreamText writes the document's plain text content to an io.Writer
func (d *Pdf) StreamText(w io.Writer) {
	logger.Info("Extracting", "pages", d.GetNPages())
	finished := make(chan bool)
	pr, pw := io.Pipe()
	go func() {
		dehyphenator.DehyphenateReaderToWriter(pr, w)
		pr.Close()
		finished <- true
	}()
	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		pw.Write([]byte(page.Text()))
		page.Close()
	}
	pw.Close()
	<-finished
}

// Metadata returns some of the PDF metadata as map with keys compatible to HTTP headers
func (d *Pdf) MetadataMap() map[string]string {
	m := make(map[string]string)
	info := d.Info()
	if info.PdfVersion != "" {
		m["x-document-version"] = info.PdfVersion
	}
	if info.Author != "" {
		m["x-document-author"] = info.Author
	}
	if info.Title != "" {
		m["x-document-title"] = info.Title
	}
	if info.Subject != "" {
		m["x-document-subject"] = info.Subject
	}
	if info.KeyWords != "" {
		m["x-document-keywords"] = info.KeyWords
	}
	if info.Pages != 0 {
		m["x-document-pages"] = strconv.Itoa(info.Pages)
	}
	if info.CreationDate != 0 {
		createTime := time.Unix(int64(info.CreationDate), 0)
		m["x-document-created"] = createTime.Format(time.RFC3339)
	}
	if info.ModificationDate != 0 {
		modTime := time.Unix(int64(info.ModificationDate), 0)
		m["x-document-modified"] = modTime.Format(time.RFC3339)
	}
	m["x-parsed-by"] = "Poppler"
	m["x-doctype"] = "pdf"
	return m
}

func (d *Pdf) DocInfo() poppler.DocumentInfo {
	return d.Info()
}
