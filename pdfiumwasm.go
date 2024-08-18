//go:build pdfium_wasm

package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/pdfdateparser"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/webassembly"
)

type Pdf struct {
	Document *responses.OpenDocument
	instance pdfium.Pdfium
}

var pool pdfium.Pool

func init() {
	pdfImplementation = "PDFium WASM"
	var err error
	pool, err = webassembly.Init(webassembly.Config{MinIdle: 2, MaxTotal: 8, ReuseWorkers: true})
	if err != nil {
		logger.Error("Error initializing WASM", "err", err)
	}
	if err != nil {
		logger.Error("Could not start PDFium worker", "err", err)
	}
}

func NewFromStream(stream io.ReadCloser) (doc *Pdf, err error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		logger.Error("Could not fully read stream when constructing PDFium document", "err", err)
	}
	stream.Close()
	return NewFromBytes(data)
}

func NewFromBytes(data []byte) (doc *Pdf, err error) {
	if mimetype.Detect(data).Extension() != ".pdf" {
		return &Pdf{nil, nil}, errors.New("not a PDF")
	}
	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		return nil, errors.New("could not obtain a PDFium worker")
	}
	pDoc, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		logger.Error("PDFium could not load PDF", "err", err)
	}
	doc = &Pdf{Document: pDoc, instance: instance}
	return
}

// Text returns the plain text content of the document
func (d *Pdf) Text() string {
	var buf strings.Builder
	instance := d.instance
	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: d.Document.Document})
	if err != nil {
		logger.Error("Could not get PDF page count", "err", err)
	}

	for n := 0; n < pageCount.PageCount; n++ {
		pIndex := &requests.PageByIndex{Document: d.Document.Document, Index: n}
		tResp, err := instance.GetPageText(&requests.GetPageText{Page: requests.Page{ByIndex: pIndex}})
		if err != nil {
			logger.Error("Could not get page text", "page", n, "err", err)
		}
		buf.WriteString(tResp.Text)
	}
	return buf.String()
}

// StreamText writes the document's plain text content to an io.Writer
func (d *Pdf) StreamText(w io.Writer) {
	finished := make(chan bool)
	pr, pw := io.Pipe()
	instance := d.instance

	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: d.Document.Document})
	if err != nil {
		logger.Error("Could not get page count", "err", err)
	}
	logger.Debug("Extracting", "pages", pageCount.PageCount)
	go func() {
		dehyphenator.DehyphenateReaderToWriter(pr, w)
		pr.Close()
		finished <- true
	}()
	for n := 0; n < pageCount.PageCount; n++ {
		pIndex := &requests.PageByIndex{Document: d.Document.Document, Index: n}
		tResp, err := instance.GetPageText(&requests.GetPageText{Page: requests.Page{ByIndex: pIndex}})
		if err != nil {
			logger.Error("Could not get page text", "err", err)
		}
		pw.Write([]byte(tResp.Text))
		// ensure there is a newline at the end of every page
		pw.Write([]byte{'\n'})
	}
	pw.Close()
	<-finished
}

// MetadataMap returns some of the PDF metadata as map with keys compatible to HTTP headers
func (d *Pdf) MetadataMap() map[string]string {
	m := make(map[string]string)
	m["x-parsed-by"] = "PDFium"
	m["x-doctype"] = "pdf"
	instance := d.instance

	if pc, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: d.Document.Document}); err == nil {
		m["x-document-pages"] = strconv.Itoa(pc.PageCount)
	}
	if versionResp, err := instance.FPDF_GetFileVersion(&requests.FPDF_GetFileVersion{Document: d.Document.Document}); err == nil && versionResp != nil && versionResp.FileVersion != 0 {
		m["x-document-version"] = fmt.Sprintf("PDF-%0.1f", float32(versionResp.FileVersion)/10.0)
	}
	if val := d.getStringField("Author"); len(val) > 0 {
		m["x-document-author"] = val
	}
	if val := d.getStringField("Title"); len(val) > 0 {
		m["x-document-title"] = val
	}
	if val := d.getStringField("Subject"); len(val) > 0 {
		m["x-document-subject"] = val
	}
	if val := d.getStringField("Keywords"); len(val) > 0 {
		m["x-document-keywords"] = val
	}
	if d := d.getDateField("ModDate"); d != "" {
		m["x-document-modified"] = d
	}
	if d := d.getDateField("CreationDate"); d != "" {
		m["x-document-created"] = d
	}
	return m
}

func (d *Pdf) getStringField(tag string) string {
	instance := d.instance
	resp, err := instance.FPDF_GetMetaText(&requests.FPDF_GetMetaText{Document: d.Document.Document, Tag: tag})
	if err == nil && resp != nil && len(resp.Value) > 0 {
		return resp.Value
	}
	// logger.Warn("metadata empty", "tag", tag, "err", err, "resp", resp)
	return ""
}

func (d *Pdf) getDateField(tag string) string {
	instance := d.instance
	resp, err := instance.FPDF_GetMetaText(&requests.FPDF_GetMetaText{Document: d.Document.Document, Tag: tag})
	if err != nil || resp.Value == "" {
		logger.Error("Retrieving PDF date failed", "tag", tag, "err", err)
		return ""
	}
	mDate, err := pdfdateparser.PdfDateToTime(resp.Value)
	if err != nil {
		logger.Error("Parsing date failed", "tag", tag, "err", err)
	}
	return mDate.Format(time.RFC3339)
}

func (d *Pdf) Close() {
	d.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: d.Document.Document})
	d.instance.Close()
}
