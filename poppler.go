//go:build !mupdf

package main

import (
	"errors"
	"io"
	"log"
	"os"
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

func NewFromStream(stream io.ReadCloser) (doc *Pdf, err error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		log.Println("NewFromStream: ", err)
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
		log.Println(err)
	}
	doc = &Pdf{pDoc}
	return
}

func NewFromPipe(r io.Reader) (Pdf, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		panic("os.Pipe(): " + err.Error())
	}

	log.Printf("Pipe FDs: reader %d; writer: %d", pr.Fd(), pw.Fd())
	go func() {
		byteCount, err2 := io.Copy(pw, r)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%d Bytes copied to pipe. Error: %v", byteCount, err2)
		pw.Close()
	}()
	pdoc, err2 := poppler.LoadFromFile(pr)
	log.Printf("Document created from FD %d.", pr.Fd())
	if err2 != nil {
		log.Fatalf("ERROR: %v, FD: %d", err2, pw.Fd())
	}
	return Pdf{pdoc}, err2
}

// Text returns the plain text content of the document
func (d *Pdf) Text() string {
	log.Printf("Number of Pages: %d", d.GetNPages())
	var buf strings.Builder
	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		buf.WriteString(dehyphenateString(page.Text()))
		page.Close()
	}
	return buf.String()
}

// StreamText writes the document's plain text content to an io.Writer
func (d *Pdf) StreamText(w io.Writer) {
	log.Printf("Number of Pages: %d", d.GetNPages())
	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		dehyph := dehyphenateString(page.Text())
		w.Write([]byte(dehyph))
		page.Close()
	}
}

// Metadata returns some of the PDF metadata as map with keys compatible to HTTP headers
func (d *Pdf) MetadataMap() map[string]string {
	m := make(map[string]string)
	if d.Info().PdfVersion != "" {
		m["x-document-version"] = d.Info().PdfVersion
	}
	if d.Info().Author != "" {
		m["x-document-author"] = d.Info().Author
	}
	if d.Info().Title != "" {
		m["x-document-title"] = d.Info().Title
	}
	if d.Info().Subject != "" {
		m["x-document-subject"] = d.Info().Subject
	}
	if d.Info().KeyWords != "" {
		m["x-document-keywords"] = d.Info().KeyWords
	}
	if d.Info().Pages != 0 {
		m["x-document-pages"] = strconv.Itoa(d.Info().Pages)
	}
	if d.Info().CreationDate != 0 {
		modTime := time.Unix(int64(d.Info().CreationDate), 0)
		m["x-document-created"] = modTime.Format(time.RFC3339)
	}
	if d.Info().ModificationDate != 0 {
		modTime := time.Unix(int64(d.Info().ModificationDate), 0)
		m["x-document-modified"] = modTime.Format(time.RFC3339)
	}
	m["x-parsed-by"] = "Poppler"
	m["x-doctype"] = "pdf"
	return m
}

func (d *Pdf) DocInfo() poppler.DocumentInfo {
	return d.Info()
}
