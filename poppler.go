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
		buf.WriteString(dehyphenateString(page.Text()))
		ch <- page
	}
	close(ch)
	return buf.String()
}

// dehyphenateString replaces hyphens at the end of a line
// with the first word from the following line, and removes
// that word from its line.
// taken from https://git.rescribe.xyz/cgit/cgit.cgi/utils/tree/cmd/dehyphenate/main.go
func dehyphenateString(in string) string {
	var newlines []string
	lines := strings.Split(in, "\n")
	for i, line := range lines {
		words := strings.Split(line, " ")
		last := words[len(words)-1]
		// the - 2 here is to account for a trailing newline and counting from zero
		if len(last) > 0 && last[len(last)-1] == '-' && i < len(lines)-2 {
			nextwords := strings.Split(lines[i+1], " ")
			if len(nextwords) > 0 {
				line = line[0:len(line)-1] + nextwords[0]
			}
			if len(nextwords) > 1 {
				lines[i+1] = strings.Join(nextwords[1:], " ")
			} else {
				lines[i+1] = ""
			}
		}
		newlines = append(newlines, line)
	}
	return strings.Join(newlines, " ")
}

//StreamText writes the document's plain text content to an io.Writer
func (d *Pdf) StreamText(w io.Writer) {
	ch := make(chan *poppler.Page, d.GetNPages())
	go closePages(ch)
	log.Printf("Number of Pages: %d", d.GetNPages())
	for n := 0; n < d.GetNPages(); n++ {
		page := d.GetPage(n)
		dehyph := dehyphenateString(page.Text())
		// pText := strings.TrimSpace(page.Text())
		w.Write([]byte(dehyph))
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
