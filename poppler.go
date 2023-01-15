package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/go-poppler"
)

type Pdf struct {
	*poppler.Document
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
func (d *Pdf) Text() (string) {
	ch := make(chan *poppler.Page, d.GetNPages())
	go closePages(ch)
	log.Printf("Number of Pages: %d", d.GetNPages())
	var buf strings.Builder

	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		buf.WriteString(strings.TrimSpace(page.Text()))
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
		pText := strings.TrimSpace(page.Text())
		w.Write([]byte(pText))
		ch <- page
	}
	close(ch)
}

func (d *Pdf) Metadata() (m map[string]interface{}) {
	tmpJson, _ := json.Marshal(d.Info())
	err := json.Unmarshal(tmpJson, &m)
	//remove Metadata xml foo
	m["Metadata"] = nil
	if err != nil {
		log.Println("Could not convert metadata: ", err)
	}
	return
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
