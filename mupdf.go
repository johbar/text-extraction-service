//go:build mupdf

package main

import (
	"io"
	"strconv"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/johbar/text-extraction-service/v2/internal/pdfdateparser"
)

type Pdf struct {
	*fitz.Document
	data *[]byte
}

func init() {
	pdfImplementation = "MuPDF (go-fitz)"
}

func NewFromBytes(data []byte) (*Pdf, error) {
	fdoc, err := fitz.NewFromMemory(data)
	if err != nil {
		logger.Error(err.Error())
		return &Pdf{nil, &data}, err
	}
	logger.Debug("Opened doc", "pages", fdoc.NumPage())
	return &Pdf{fdoc, &data}, err
}

func (d *Pdf) Text() string {
	result := ""
	for i := 0; i <= d.NumPage(); i++ {
		pText, err := d.Document.Text(i)
		if err != nil {
			logger.Error(err.Error())
		}
		result += pText
	}
	return result
}

func (d *Pdf) StreamText(w io.Writer) {
	for n := 0; n < d.NumPage(); n++ {
		pageText, err := d.Document.Text(n)
		if err != nil {
			logger.Error("MuPDF failed when extracting text", "page", n, "err", err)
			continue
		}
		WriteTextOrRunOcrOnPage(pageText, n, w, d.data)
	}
}

func (d *Pdf) GetNPages() int {
	return d.Document.NumPage()
}

func (d *Pdf) MetadataMap() map[string]string {
	m := d.Document.Metadata()
	r := make(map[string]string)

	if val := stripNulls(m["format"]); val != "" {
		r["x-document-version"] = val
	}
	if val := stripNulls(m["author"]); val != "" {
		r["x-document-author"] = val
	}
	if val := stripNulls(m["title"]); val != "" {
		r["x-document-title"] = val
	}
	if val := stripNulls(m["subject"]); val != "" {
		r["x-document-subject"] = val
	}
	if val := stripNulls(m["keywords"]); val != "" {
		r["x-document-keywords"] = val
	}
	r["x-document-pages"] = strconv.Itoa(d.NumPage())

	if d, _ := pdfdateparser.PdfDateToIso(stripNulls(m["creationDate"])); len(d) > 0 {
		r["x-document-created"] = d
	}
	if d, _ := pdfdateparser.PdfDateToIso(stripNulls(m["modDate"])); len(d) > 0 {
		r["x-document-modified"] = d
	}
	if val := stripNulls(m["producer"]); val != "" {
		r["x-document-producer"] = val
	}
	if val := stripNulls(m["creator"]); val != "" {
		r["x-document-creator"] = val
	}
	r["x-parsed-by"] = "MuPDF"
	return r
}

// go-fitz seems to return Strings of 256 length with zero byte padding
func stripNulls(val string) string {
	return strings.ReplaceAll(val, "\u0000", "")
}

func (d Pdf) Close() {
	d.Document.Close()
	d.data = nil
}
