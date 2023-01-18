//go:build !poppler
package main

import (
	"io"
	"log"

	"github.com/gen2brain/go-fitz"
)

type Pdf struct {
	*fitz.Document
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
		w.Write([]byte(pText))
	}
}

func (d *Pdf) GetNPages() int {
	return d.Document.NumPage()
}
