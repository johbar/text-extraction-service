// Package docparser extracts plain text from PowerPoint (.ppt) and Word Binary Format (.doc) files.
//
// It implements the MS-DOC text-retrieval algorithm:
//   - Parses the FIB (File Information Block) from the WordDocument stream
//   - Locates the Clx in the Table stream via FibRgFcLcb97.fcClx
//   - Walks the PlcPcd (piece table) to collect every text run
//   - Decodes each run as UTF-16LE (uncompressed) or Windows-1252 (compressed)
//   - Skips non-text CPs: field markers, object anchors, picture placeholders
//
// References:
//
//	[MS-DOC] §2.4.1  Retrieving Text
//	[MS-DOC] §2.5.1  Fib / FibBase / FibRgFcLcb97
//	[MS-DOC] §2.9.73 FcCompressed
//	[MS-DOC] §3.1    Example of a Clx
package docparser

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"time"
)

type WordDoc struct {
	data       *[]byte
	docStreams *docStreams
	f          *os.File
	path       string
}

func NewFromBytes(data []byte) (*WordDoc, error) {
	buf := bytes.NewReader(data)
	ds, err := openDocStreams(buf)
	if err != nil {
		return nil, err
	}
	doc := &WordDoc{data: &data, docStreams: ds}
	return doc, err
}

func Open(path string) (*WordDoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	ds, err := openDocStreams(f)
	if err != nil {
		return nil, err
	}
	return &WordDoc{f: f, path: path, docStreams: ds}, nil
}

func (d *WordDoc) Pages() int {
	return -1
}

func (d *WordDoc) Path() string {
	return d.path
}

func (d *WordDoc) Data() *[]byte {
	return d.data
}

func (d *WordDoc) MetadataMap() map[string]string {
	metadata, err := d.docStreams.metadata()
	if err != nil {
		return map[string]string{}
	}
	m := map[string]string{
		"x-doctype":           "msword",
		"x-document-author":   metadata.Author,
		"x-document-category": metadata.Category,
		"x-document-company":  metadata.Company,
		"x-document-keywords": metadata.Keywords,
		"x-document-manager":  metadata.Manager,
		"x-document-subject":  metadata.Subject,
		"x-document-title":    metadata.Title,
	}
	if !metadata.Created.IsZero() {
		m["x-document-created"] = metadata.Created.Format(time.RFC3339)
	}
	if !metadata.LastSaved.IsZero() {
		m["x-document-modified"] = metadata.LastSaved.Format(time.RFC3339)
	}
	if metadata.PageCount != 0 {
		m["x-document-pages"] = strconv.Itoa(int(metadata.PageCount))
	}
	if metadata.CharCount != 0 {
		m["x-document-chars"] = strconv.Itoa(int(metadata.CharCount))
	}
	if metadata.WordCount != 0 {
		m["x-document-words"] = strconv.Itoa(int(metadata.WordCount))
	}
	// omit empty
	for k, v := range m {
		if v == "" {
			delete(m, k)
		}
	}
	return m
}

func (d *WordDoc) Text(_ int) (string, bool) {
	panic("not allowed")
}

func (d *WordDoc) StreamText(w io.Writer) error {
	if d.docStreams.wordDocSize != 0 {
		return writeText(d.docStreams, w)
	}
	return extractSlides(d.docStreams.pptDoc, d.docStreams.currentUser, func(st SlideText) error {
		_, err := w.Write([]byte(st.Text))
		return err
	})
}

func (d *WordDoc) HasNewlines() bool {
	return true
}

func (d *WordDoc) Close() {
	if d.f != nil {
		d.f.Close()
	}
}
