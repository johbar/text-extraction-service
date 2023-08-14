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

func NewFromBytes(data []byte) (*Pdf, error) {
	fdoc, err := fitz.NewFromMemory(data)
	if err != nil {
		log.Println(err.Error())
	}
	log.Printf("Opened Doc with %d Pages", fdoc.NumPage())
	return &Pdf{fdoc}, err
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

func (d *Pdf) MetadataMap() map[string]string {
	m := d.Document.Metadata()
	r := make(map[string]string)
	if m["format"] != "" {
		r["x-document-version"] = m["format"]
	}
	if m["author"] != "" {
		r["x-document-author"] = m["author"]
	}
	if m["title"] != "" {
		r["x-document-title"] = m["title"]
	}
	if m["subject"] != "" {
		r["x-document-subject"] = m["subject"]
	}
	if m["keywords"] != "" {
		r["x-document-keywords"] = m["keywords"]
	}

	r["x-document-pages"] = strconv.Itoa(d.NumPage())

	// dates are in strange format
	// FIXME: parse dates
	if m["creationDate"] != "" {
		r["x-document-created"] = m["creationDate"]
	}
	if m["modDate"] != "" {
		r["x-document-modified"] = m["modDate"]
	}
	if m["producer"] != "" {
		r["x-document-producer"] = m["producer"]
	}
	if m["creator"] != "" {
		r["x-document-creator"] = m["creator"]
	}
	r["x-parsed-by"] = "MuPDF"
	return r
}


func (d Pdf) Close () {
	d.Close()
}