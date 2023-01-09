package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"

	"github.com/johbar/go-poppler"
)

type PopplerPdf struct {
	*poppler.Document
}

func NewFromStream(stream io.ReadCloser) (doc PopplerPdf, err error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		log.Println("NewFromStream: ", err)
	}
	defer stream.Close()
	return NewFromBytes(data)
}

func NewFromBytes(data []byte) (doc PopplerPdf, err error) {
	pDoc, err := poppler.Load(data)
	if err != nil {
		log.Println(err)
	}
	doc = PopplerPdf{pDoc}
	return
}

//Text returns the plain text content of the document
func (d *PopplerPdf) Text() string {

	buf := bytes.NewBufferString("")
	d.StreamText(buf)
	return buf.String()
}

//StreamText writes the document's plain text content to an io.Writer
func (d *PopplerPdf) StreamText(w io.Writer) {
	ch := make (chan *poppler.Page, 100)
	go closePages(ch)
	log.Printf("Number of Pages: %d", d.GetNPages())
	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		w.Write([]byte(page.Text()))
		ch <- page
	}
	close(ch)
}

func (d *PopplerPdf) HasMetadata() bool {
	return true
}

func (d *PopplerPdf) Metadata() *map[string]interface{} {
	var m map[string]interface{}
	tmpJson, _ := json.Marshal(d.Info())
	err := json.Unmarshal(tmpJson, &m)
	//remove Metadata xml foo
	m["Metadata"] = nil
	if err != nil {
		log.Println("Could not convert metadata: ", err)
	}
	return &m
}

func (d *PopplerPdf) DocInfo() poppler.DocumentInfo {
	return d.Info()
}

func closePages(ch chan *poppler.Page) {
	for page := range ch {
		if page != nil {
			page.Close()
		}
	}
}