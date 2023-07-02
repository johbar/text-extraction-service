//go:build mupdf
package main

import (
	"io"
	"log"
	"strconv"

	"github.com/gen2brain/go-fitz"
)

type Pdf struct {
	*fitz.Document
}

func init() {
	println("Using MuPDF (go-fitz) library.")
}

func NewFromBytes(data []byte) (Pdf, error) {
	fdoc, err := fitz.NewFromMemory(data)
	if err != nil {
		log.Println(err.Error())
	}
	log.Printf("Opened Doc with %d Pages", fdoc.NumPage())
	return Pdf{fdoc}, err
}

func NewFromStream(stream io.Reader) (Pdf, error) {
	fdoc, err := fitz.NewFromReader(stream)
	if err != nil {
		log.Println(err.Error())
	}
	log.Printf("Opened Doc with %d Pages", fdoc.NumPage())
	return Pdf{fdoc}, err
}

func (d *Pdf) Text() string {
	result := ""
	for i := 0; i <= d.NumPage(); i++ {
		pText, err := d.Document.Text(i)
		if err != nil {
			log.Println(err.Error())
		}
		result += pText
	}
	return result
}

func (d *Pdf) StreamText(w io.Writer) {
	for i := 0; i < d.NumPage(); i++ {
		pText, err := d.Document.Text(i)
		if err != nil {
			log.Println(err.Error())
		}
		text := dehyphenateString(pText)
		w.Write([]byte(text))
	}
}

func (d *Pdf) GetNPages() int {
	return d.Document.NumPage()
}

func (d *Pdf) Metadata() Metadata {
	m := d.Document.Metadata()
	r := make(Metadata)
	if m["format"] != "" {
		r["x-pdf-version"] = m["format"]
	}
	if m["author"] != "" {
		r["x-pdf-author"] = m["author"]
	}
	if m["title"] != "" {
		r["x-pdf-title"] = m["title"]
	}
	if m["subject"] != "" {
		r["x-pdf-subject"] = m["subject"]
	}
	if m["keywords"] != "" {
		r["x-pdf-keywords"] = m["keywords"]
	}

	r["x-pdf-pages"] = strconv.Itoa(d.NumPage())

	// dates are in strange format
	// FIXME: parse dates
	if m["creationDate"] != "" {
		r["x-pdf-created"] = m["creationDate"]
	}
	if m["modDate"] != "" {
		r["x-pdf-modified"] = m["modDate"]
	}
	if m["producer"] != "" {
		r["x-pdf-producer"] = m["producer"]
	}
	if m["creator"] != "" {
		r["x-pdf-creator"] = m["creator"]
	}
	r["x-parsed-by"] = "MuPDF"
	return r
}
