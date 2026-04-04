package rtfparser

import (
	"bytes"
	"io"
	"os"
	"strings"
	"time"
)

type RichTextDoc struct {
	data  []byte
	input io.ReadSeekCloser
	path  string
}

type closableBytesReader struct{ *bytes.Reader }

func (c closableBytesReader) Close() error { return nil }

func NewFromBytes(data []byte) (d *RichTextDoc, err error) {
	input := bytes.NewReader(data)
	ci := closableBytesReader{input}
	d = &RichTextDoc{data: data, input: ci}
	return
}

func Open(path string) (*RichTextDoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return &RichTextDoc{input: f, path: path}, nil
}

// Close is a no-op for RTFs
func (d *RichTextDoc) Close() {
	d.input.Close()
}

func (d *RichTextDoc) HasNewlines() bool {
	return true
}
func (d *RichTextDoc) Pages() int {
	return -1
}

func (d *RichTextDoc) Path() string {
	return d.path
}

func (d *RichTextDoc) Data() *[]byte {
	return nil
}

func (d *RichTextDoc) Text(_ int) (string, bool) {
	var sb strings.Builder
	_ = Convert(d.input, &sb)
	return sb.String(), false
}

func (d *RichTextDoc) StreamText(w io.Writer) error {
	return Convert(d.input, w)
}

func (d *RichTextDoc) MetadataMap() map[string]string {
	m := make(map[string]string)
	metadata, err := ExtractMetadata(d.input)
	d.input.Seek(0, io.SeekStart)
	if err != nil {
		//FIXME
		println(err.Error())
		return m
	}
	if metadata.Author != "" {
		m["x-document-author"] = metadata.Author
	}
	if metadata.Category != "" {
		m["x-document-category"] = metadata.Category
	}
	if metadata.Comment != "" {
		m["x-document-comment"] = metadata.Comment
	}
	if metadata.Company != "" {
		m["x-document-company"] = metadata.Company
	}
	if metadata.Operator != "" {
		m["x-document-operator"] = metadata.Operator
	}

	if metadata.Subject != "" {
		m["x-document-subject"] = metadata.Subject
	}
	if metadata.Title != "" {
		m["x-document-title"] = metadata.Title
	}
	if metadata.Created != nil {
		m["x-document-created"] = metadata.Created.Format(time.RFC3339)
	}
	if metadata.Modified != nil {
		m["x-document-modified"] = metadata.Modified.Format(time.RFC3339)
	}
	return m
}
