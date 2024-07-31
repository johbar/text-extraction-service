/*
Package docparser implements a parser for legacy MS Word documents (.doc).
It depends on the wvWare tool which must be installed.

The metadata parser is mainly taken from https://github.com/sajari/docconv/blob/master/doc.go
*/
package docparser

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/richardlehane/mscfb"
	"github.com/richardlehane/msoleps"
	"github.com/richardlehane/msoleps/types"
)

// RegExp matching ugly control characters (possibly surrounded by whitespace)
// and duplicate whitespace
var reCleaner = regexp.MustCompile(`\s*?\pC\pS*?|\s{2,}`)

// Initialized indicates if the package is usable (depending on the presence of the WV tool)
var Initialized bool

type DocMetadata struct {
	Author   string `json:"author,omitempty"`
	Category string `json:"category,omitempty"`
	Comment  string `json:"comment,omitempty"`
	Company  string `json:"company,omitempty"`
	Keywords string `json:"keywords,omitempty"`
	Manager  string `json:"manager,omitempty"`
	Subject  string `json:"subject,omitempty"`
	Title    string `json:"title,omitempty"`

	Created   *time.Time `json:"created,omitempty"`
	Modified  *time.Time `json:"modified,omitempty"`
	PageCount int32      `json:"page_count,omitempty"`
	CharCount int32      `json:"char_count,omitempty"`
	WordCount int32      `json:"word_count,omitempty"`
}

type WordDoc struct {
	metadata DocMetadata
	data     *[]byte
	text     string
}

func init(){
	_, err := exec.LookPath("wvWare")
	if err != nil {
		Initialized = false
		return
	}
	Initialized = true
}

func NewFromBytes(data []byte) (doc *WordDoc, err error) {
	doc = &WordDoc{data: &data}
	buf := bytes.NewReader(data)
	doc.metadata, err = readMetadata(buf)
	if err != nil {
		slog.Error("docparser: Metadata extraction failed",  "err", err)
	}
	return
}

func NewFromStream(stream io.ReadCloser) (doc *WordDoc, err error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		slog.Error("docparser: could not read stream", "err", err)
	}
	// stream.Close()
	return NewFromBytes(data)
}

func readMetadata(r io.ReaderAt) (m DocMetadata, err error) {
	defer func() {
		if e := recover(); e != nil {
			slog.Error("docparser: panic when reading doc format", "err", e)
		}
	}()

	doc, err := mscfb.New(r)
	if err != nil {
		slog.Error("docparser: could not read doc", "err", err)
		return m, err
	}

	props := msoleps.New()

	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		if msoleps.IsMSOLEPS(entry.Initial) {
			if oerr := props.Reset(doc); oerr != nil {
				slog.Error("docparser: could not reset props", "err", oerr)
				err = oerr
				break
			}

			for _, prop := range props.Property {
				// String values:
				switch prop.Name {
				case "Author":
					m.Author = prop.String()
				case "Category":
					m.Category = prop.String()
				case "Comments":
					m.Comment = prop.String()
				case "Company":
					m.Company = prop.String()
				case "Keywords":
					m.Keywords = prop.String()
				case "Manager":
					m.Manager = prop.String()
				case "Subject":
					m.Subject = prop.String()
				case "Title":
					m.Title = prop.String()
				}
				if d, ok := prop.T.(types.FileTime); ok {
					switch prop.Name {
					case "CreateTime":
						t := d.Time()
						m.Created = &t
					case "LastSaveTime":
						t := d.Time()
						m.Modified = &t
					}
				} else if i, ok := prop.T.(types.I4); ok {
					switch prop.Name {
					case "PageCount":
						m.PageCount = int32(i)
					case "Character count":
						m.CharCount = int32(i)
					case "WordCount":
						m.WordCount = int32(i)
					}
				}
			}
		}
	}
	return m, err
}

func (d *WordDoc) Metadata() DocMetadata {
	return d.metadata
}

func (d *WordDoc) MetadataMap() map[string]string {
	m := map[string]string{
		"x-doctype":           "msword",
		"x-document-author":   d.metadata.Author,
		"x-document-category": d.metadata.Category,
		"x-document-comment":  d.metadata.Comment,
		"x-document-company":  d.metadata.Company,
		"x-document-keywords": d.metadata.Keywords,
		"x-document-manager":  d.metadata.Manager,
		"x-document-subject":  d.metadata.Subject,
		"x-document-title":    d.metadata.Title,
	}
	if d.metadata.Created != nil {
		m["x-document-created"] = d.metadata.Created.Format(time.RFC3339)
	}
	if d.metadata.Modified != nil {
		m["x-document-modified"] = d.metadata.Modified.Format(time.RFC3339)
	}
	if d.metadata.PageCount != 0 {
		m["x-document-page-count"] = strconv.Itoa(int(d.metadata.PageCount))
	}
	if d.metadata.CharCount != 0 {
		m["x-document-char-count"] = strconv.Itoa(int(d.metadata.CharCount))
	}
	if d.metadata.WordCount != 0 {
		m["x-document-word-count"] = strconv.Itoa(int(d.metadata.WordCount))
	}
	// omit empty
	for k, v := range m {
		if v == "" {
			delete(m, k)
		}
	}
	return m
}

func (d *WordDoc) Text() string {
	if d.text == "" {
		buf := bytes.NewBuffer(*d.data)
		d.text = doc2text(buf)
	}
	return d.text
}

func (d *WordDoc) StreamText(w io.Writer) {
	if d.text != "" {
		w.Write([]byte(d.text))
	} else {

		cmd := exec.Command("wvWare", "-x", "/usr/share/wv/wvText.xml", "-1", "-c", "utf-8", "/dev/stdin")

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			slog.Error("docparser: could not connect stdout", "err", err)
		}
		s := bufio.NewScanner(stdout)
		cmd.Stdin = bytes.NewBuffer(*d.data)
		err = cmd.Start()
		if err != nil {
			slog.Error("docparser: could not start wvWare", "err", err)
			if exitErr, ok := err.(*exec.ExitError); ok {
				slog.Error("docpaser: wvWare failed", "err", exitErr.Stderr)
			}
		}

		for s.Scan() {
			line := s.Text() + " "
			line = reCleaner.ReplaceAllLiteralString(line, " ")
			// don't add empty lines
			if line != "" && line != " " {
				w.Write([]byte(line))
			}
		}
		err = cmd.Wait()
		if err != nil {
			slog.Error("docparser failed", "err", err)
			if exitErr, ok := err.(*exec.ExitError); ok {
				slog.Error("docparser: wvWare failed", "err", exitErr.Stderr)
			}
		}
	}
}

func doc2text(r io.Reader) string {
	// cmd := exec.Command("antiword", "-w0", "-i1", "-")
	cmd := exec.Command("wvWare", "-x", "/usr/share/wv/wvText.xml", "-1", "-c", "utf-8", "/dev/stdin")
	cmd.Stdin = r
	out, err := cmd.Output()
	if err != nil {
		slog.Error("docparser failed", "err", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			slog.Error("docparser: wvWare failed", "err", exitErr.Stderr)
		}
	}
	return string(out)
}

// Close is a no-op
func (d *WordDoc) Close() {
}
