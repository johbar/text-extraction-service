//go:build mupdf

package main

import (
	"io"
	"strconv"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/johbar/text-extraction-service/v2/pkg/dehyphenator"
	"github.com/johbar/text-extraction-service/v2/pkg/pdfdateparser"
)

type Pdf struct {
	*fitz.Document
}

func init() {
	pdfImplementation = "MuPDF (go-fitz)"
}

func NewFromBytes(data []byte) (*Pdf, error) {
	fdoc, err := fitz.NewFromMemory(data)
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Debug("Opened doc", "pages", fdoc.NumPage())
	return &Pdf{fdoc}, err
}

func NewFromStream(stream io.Reader) (Pdf, error) {
	fdoc, err := fitz.NewFromReader(stream)
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Debug("Opened Doc", "Pages", fdoc.NumPage())
	return Pdf{fdoc}, err
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
	pr, pw := io.Pipe()
	finished := make(chan bool)
	go func() {
		dehyphenator.DehyphenateReaderToWriter(pr, w)
		pr.Close()
		finished <- true
	}()
	for i := 0; i < d.NumPage(); i++ {
		pText, err := d.Document.Text(i)
		if err != nil {
			logger.Error(err.Error())
			continue
		}
		pw.Write([]byte(pText))
	}
	pw.Close()
	<-finished
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

	if d, err := pdfdateparser.PdfDateToIso(stripNulls(m["creationDate"])); err == nil {
		r["x-document-created"] = d
	} else {
		logger.Warn("invalid creationDate", "err", err)
	}
	if d, err := pdfdateparser.PdfDateToIso(stripNulls(m["modDate"])); err == nil {
		r["x-document-modified"] = d
	} else {
		logger.Warn("invalid modDate", "err", err, "val", stripNulls(m["modDate"]))
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

func stripNulls(val string) string {
	return strings.ReplaceAll(val, "\u0000", "")
}

func (d Pdf) Close() {
	d.Document.Close()
}
