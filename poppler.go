//go:build !mupdf
package main

import (
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/go-poppler"
)

type Pdf struct {
	*poppler.Document
}

func init() {
	println("Using Poppler (GLib) library. Version:", poppler.Version())
	
}

func NewFromStream(stream io.ReadCloser) (doc Pdf, err error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		log.Println("NewFromStream: ", err)
	}
	defer stream.Close()
	return NewFromBytes(data)
}

func NewFromBytes(data []byte) (doc Pdf, err error) {
	if mimetype.Detect(data).Extension() != ".pdf" {
		return Pdf{nil}, errors.New("not a PDF")
	}
	pDoc, err := poppler.Load(data)
	if err != nil {
		log.Println(err)
	}
	doc = Pdf{pDoc}
	return
}

//Text returns the plain text content of the document
func (d *Pdf) Text() string {
	ch := make(chan *poppler.Page, d.GetNPages())
	go closePages(ch)
	log.Printf("Number of Pages: %d", d.GetNPages())
	var buf strings.Builder

	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		buf.WriteString(dehyphenateString(page.Text()))
		ch <- page
	}
	close(ch)
	return buf.String()
}

//StreamText writes the document's plain text content to an io.Writer
func (d *Pdf) StreamText(w io.Writer) {
	ch := make(chan *poppler.Page, d.GetNPages())
	go closePages(ch)
	log.Printf("Number of Pages: %d", d.GetNPages())
	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		dehyph := dehyphenateString(page.Text())
		w.Write([]byte(dehyph))
		ch <- page
	}
	close(ch)
}

//Metadata returns some of the PDF metadata as map with keys compatible to HTTP headers
func (d *Pdf) Metadata() Metadata {
	m := make(map[string]string)
	if d.Info().PdfVersion != "" {
		m["x-pdf-version"] = d.Info().PdfVersion
	}
	if d.Info().Author != "" {
		m["x-pdf-author"] = d.Info().Author
	}
	if d.Info().Title != "" {
		m["x-pdf-title"] = d.Info().Title
	}
	if d.Info().Subject != "" {
		m["x-pdf-subject"] = d.Info().Subject
	}
	if d.Info().KeyWords != "" {
		m["x-pdf-keywords"] = d.Info().KeyWords
	}
	if d.Info().Pages != 0 {
		m["x-pdf-pages"] = strconv.Itoa(d.Info().Pages)
	}
	if d.Info().CreationDate != 0 {
		modTime := time.Unix(int64(d.Info().CreationDate), 0)
		m["x-pdf-created"] = modTime.Format(time.RFC3339)
	}
	if d.Info().ModificationDate != 0 {
		modTime := time.Unix(int64(d.Info().ModificationDate), 0)
		m["x-pdf-modified"] = modTime.Format(time.RFC3339)
	}
	m["x-parsed-by"] = "Poppler"
	return m
}

func (d *Pdf) DocInfo() poppler.DocumentInfo {
	return d.Info()
}

func closePages(ch chan *poppler.Page) {
	for page := range ch {
		if page != nil {
			page.Close()
		}
	}
}
