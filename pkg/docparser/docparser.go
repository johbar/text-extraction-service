/*
Package docparser implements a parser for legacy MS Word documents (.doc).
It depends on the wvWare tool which must be installed.

The metadata parser is mainly taken from https://github.com/sajari/docconv/blob/master/doc.go
*/
package docparser

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/richardlehane/mscfb"
	"github.com/richardlehane/msoleps"
	"github.com/richardlehane/msoleps/types"
)

var (
	// RegExp matching ugly control characters (possibly surrounded by whitespace)
	// and duplicate whitespace
	reCleaner = regexp.MustCompile(`\s*?\pC\pS*?|\s{2,}`)

	// Initialized indicates if the package is usable (depending on the presence of the wv and/or antiword)
	Initialized bool = false

	wvWare   = progAndArgs{cmd: "wvWare", args: []string{"-x", "/usr/share/wv/wvText.xml", "-1", "-c", "utf-8"}, stdin: "/dev/stdin"}
	antiword = progAndArgs{cmd: "antiword", args: []string{"-w", "0", "-m", "UTF-8"}, stdin: "-"}
	catdoc   = progAndArgs{cmd: "catdoc", args: []string{"-w", "-d", "utf-8"}, stdin: "-"}

	wordProcessor progAndArgs
)

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
	path     string
}

type progAndArgs struct {
	cmd   string
	args  []string
	stdin string
}

func init() {
	for _, prog := range []progAndArgs{antiword, wvWare, catdoc} {
		if _, err := exec.LookPath(prog.cmd); err == nil {
			wordProcessor = prog
			Initialized = true
			return
		}
	}
}

func NewFromBytes(data []byte) (*WordDoc, error) {
	buf := bytes.NewReader(data)
	metadata, err := readMetadata(buf)
	doc := &WordDoc{data: &data, metadata: metadata}
	return doc, err
}

func Open(path string) (*WordDoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	metadata, err := readMetadata(f)
	doc := &WordDoc{path: path, metadata: metadata}
	return doc, err
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

func readMetadata(r io.ReaderAt) (DocMetadata, error) {
	defer func() {
		if e := recover(); e != nil {
			println("docparser: panic when reading doc format", "err", e)
		}
	}()
	var err error
	var m DocMetadata
	doc, err := mscfb.New(r)
	if err != nil {
		return m, err
	}

	props := msoleps.New()

	for entry, err2 := doc.Next(); err2 == nil; entry, err2 = doc.Next() {
		if msoleps.IsMSOLEPS(entry.Initial) {
			if oerr := props.Reset(doc); oerr != nil {
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
		m["x-document-pages"] = strconv.Itoa(int(d.metadata.PageCount))
	}
	if d.metadata.CharCount != 0 {
		m["x-document-chars"] = strconv.Itoa(int(d.metadata.CharCount))
	}
	if d.metadata.WordCount != 0 {
		m["x-document-words"] = strconv.Itoa(int(d.metadata.WordCount))
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
	return d.runExternalWordProcessor(w, wordProcessor)
}

func (d *WordDoc) runExternalWordProcessor(w io.Writer, wordProg progAndArgs) error {
	cmd := exec.Command(wordProg.cmd, wordProg.args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if len(d.path) < 1 {
		cmd.Stdin = bytes.NewReader(*d.data)
		cmd.Args = append(cmd.Args, wordProg.stdin)
	} else {
		cmd.Args = append(cmd.Args, d.path)
	}

	s := bufio.NewScanner(stdout)
	err = cmd.Start()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			err = errors.Join(err, errors.New(string(exitErr.Stderr)))
		}
		return err
	}

	for s.Scan() {
		line := s.Text() + " "
		line = reCleaner.ReplaceAllLiteralString(line, " ")
		// don't add empty lines
		if line != "" && line != " " {
			_, err := w.Write([]byte(line))
			if err != nil {
				return err
			}
		}
	}
	err = cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr
		}
	}
	return nil
}

// Close is a no-op
func (d *WordDoc) Close() {
}
