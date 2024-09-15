//go:build pdfium

package main

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/johbar/text-extraction-service/v2/internal/pdfdateparser"
	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/responses"
	"github.com/klippa-app/go-pdfium/single_threaded"
)

type Pdf struct {
	*responses.OpenDocument
	data *[]byte
}

var pool pdfium.Pool
var instance pdfium.Pdfium

func init() {
	pdfImplementation = "PDFium"
	pool = single_threaded.Init(single_threaded.Config{})
	var err error
	instance, err = pool.GetInstance(time.Second * 3)
	if err != nil {
		logger.Error("Could not start PDFium worker", "err", err)
	}
}

func NewFromBytes(data []byte) (doc *Pdf, err error) {
	pDoc, err := instance.OpenDocument(&requests.OpenDocument{File: &data})
	if err != nil {
		logger.Error("PDFium could not load PDF", "err", err)
	}
	doc = &Pdf{pDoc, &data}
	return
}

func (d *Pdf) PageCount() int {
	pageCount, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: d.Document})
	if err != nil {
		logger.Error("Could not get PDF page count", "err", err)
	}
	return pageCount.PageCount
}

// Text returns the plain text content of the document
func (d *Pdf) PageText(pagenum int) string {
	pIndex := &requests.PageByIndex{Document: d.Document, Index: pagenum}
	tResp, err := instance.GetPageText(&requests.GetPageText{Page: requests.Page{ByIndex: pIndex}})
	if err != nil {
		logger.Error("Could not get page text", "page", pagenum, "err", err)
		return ""
	}
	return tResp.Text
}

// StreamText writes the document's plain text content to an io.Writer
func (d *Pdf) StreamText(w io.Writer) {
	pageCount := d.PageCount()
	logger.Debug("Extracting", "pages", pageCount)
	for n := 0; n < pageCount; n++ {
		pageText := d.PageText(n)
		WriteTextOrRunOcrOnPage(pageText, n, w, d.data)
	}
}

func (d *Pdf) renderPage(n int) []byte {
	resp, err := instance.RenderToFile(&requests.RenderToFile{
		RenderPageInDPI: &requests.RenderPageInDPI{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: d.Document,
					Index:    n}},
			DPI: 200,
		},
		OutputFormat:  requests.RenderToFileOutputFormatJPG,
		OutputTarget:  requests.RenderToFileOutputTargetBytes,
		OutputQuality: 100,
	})
	if err != nil {
		logger.Error("Could not render", "page", n)
	}
	img := resp.ImageBytes

	return *img
}

// MetadataMap returns some of the PDF metadata as map with keys compatible to HTTP headers
func (d *Pdf) MetadataMap() map[string]string {
	m := make(map[string]string)
	m["x-parsed-by"] = "PDFium"
	m["x-doctype"] = "pdf"

	if pc, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: d.Document}); err == nil {
		m["x-document-pages"] = strconv.Itoa(pc.PageCount)
	}
	if versionResp, err := instance.FPDF_GetFileVersion(&requests.FPDF_GetFileVersion{Document: d.Document}); err == nil && versionResp != nil && versionResp.FileVersion != 0 {
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
	resp, err := instance.FPDF_GetMetaText(&requests.FPDF_GetMetaText{Document: d.Document, Tag: tag})
	if err != nil || resp == nil || len(resp.Value) == 0 {
		return ""
	}
	return resp.Value
}

func (d *Pdf) getDateField(tag string) string {
	resp, err := instance.FPDF_GetMetaText(&requests.FPDF_GetMetaText{Document: d.Document, Tag: tag})
	if err != nil || resp.Value == "" {
		return ""
	}
	mDate, err := pdfdateparser.PdfDateToTime(resp.Value)
	if err != nil {
		logger.Warn("Parsing date failed", "tag", tag, "err", err)
	}
	return mDate.Format(time.RFC3339)
}

func (d *Pdf) Close() {
	instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: d.Document})
	d.data = nil
}
