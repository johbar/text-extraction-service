//go:build !mupdf

package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/johbar/go-poppler"
	"golang.org/x/sys/unix"
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
	stream.Close()
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

func NewFromPipe(r io.Reader) (doc Pdf, err error) {

	pr, pw, err := os.Pipe()
	if err != nil {
		panic("os.Pipe(): " + err.Error())
	}

	log.Printf("Pipe FD: %d and %d", pr.Fd(), pw.Fd())
	ch := make(chan *poppler.Document, )
	go func() {
		pdoc, err := poppler.LoadFromFd(pr.Fd())
		log.Printf("Document created from FD %d. Error: %v", pr.Fd(), err)
		ch <- pdoc
	}()
	byteCount, err2 := io.Copy(pw, r)
	log.Printf("Copy to pw finished after %d bytes. Error: %v", byteCount, err2)
	if err2 != nil {
		log.Printf("ERROR: %v, FD: %d", err2, pw.Fd())
		return Pdf{nil}, err2
	}
	pw.Close()
	// time.Sleep(time.Microsecond*100)
	log.Printf("Waiting for Poppler doc to arrive...")
	pdoc := <-ch
	return Pdf{pdoc}, err
}

func NewFromFifo(r io.Reader) (doc Pdf, err error) {
	// log.Print("NewFromFifo entered.")
	fifoPath := fmt.Sprintf("%s/poppler-%d.fifo", os.TempDir(), rand.Int())
	fifoErr := unix.Mkfifo(fifoPath, 0666)
	if fifoErr != nil {
		panic(fifoErr)
	}
	defer os.Remove(fifoPath)
	go func() {
		// log.Print("Goroutine entered.")
		wfifo, wfifoErr := os.OpenFile(fifoPath, os.O_WRONLY, 0)
		if wfifoErr != nil {
			panic(wfifoErr)
		}
		// log.Print("wfifo created.")
		byteCount, err := io.Copy(wfifo, r)
		log.Printf("io.Copy() to %s done: %d", fifoPath, byteCount)
		if err != nil {
			log.Printf("ERROR: %v, FD: %d", err, wfifo.Fd())
			// panic(err)
		}
		wfifo.Close()
	}()
	rfifo, rfifoErr := os.OpenFile(fifoPath, os.O_RDONLY, 0)
	if rfifoErr != nil {
		panic(rfifoErr)
	}
	// log.Print("rfifo created.")
	pdoc, err := poppler.LoadFromFd(rfifo.Fd())
	doc = Pdf{pdoc}
	log.Printf("Document created from FD %d, %s, %v", rfifo.Fd(), fifoPath, err)
	return
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
func (d *Pdf) Metadata() PdfMetadata {
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
