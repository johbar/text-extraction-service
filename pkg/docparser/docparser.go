/*
Package docparser implements a parser for legacy MS Word documents (.doc).
It depends on the wvWare tool which must be installed.

The metadata parser is mainly taken from https://github.com/sajari/docconv/blob/master/doc.go,
but 
*/
package docparser

import (
	"bytes"
	"io"
	"log"
	"os/exec"
	"time"

	"github.com/richardlehane/mscfb"
	"github.com/richardlehane/msoleps"
	"github.com/richardlehane/msoleps/types"
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

	Created   time.Time `json:"created,omitempty"`
	Modified  time.Time `json:"modified,omitempty"`
	PageCount int32     `json:"page_count,omitempty"`
	CharCount int32     `json:"char_count,omitempty"`
	WordCount int32     `json:"word_count,omitempty"`
}

type WordDoc struct {
	metadata DocMetadata
	data     *[]byte
	text     string
}

func NewFromBytes(data []byte) (doc *WordDoc, err error) {
	doc = &WordDoc{data: &data}
	buf := bytes.NewReader(data)
	doc.metadata, err = readMetadata(buf)
	if err != nil {
		log.Fatal(err)
	}
	return
}

func NewFromStream(stream io.ReadCloser) (doc *WordDoc, err error) {
	data, err := io.ReadAll(stream)
	if err != nil {
		log.Println("NewFromStream: ", err)
	}
	// stream.Close()
	return NewFromBytes(data)
}

func readMetadata(r io.ReaderAt) (m DocMetadata, err error) {
	defer func() {
		if e := recover(); e != nil {
			log.Printf("panic when reading doc format: %v", e)
		}
	}()

	doc, err := mscfb.New(r)
	if err != nil {
		log.Printf("docparser: could not read doc: %v", err)
		return m, err
	}

	props := msoleps.New()

	for entry, err := doc.Next(); err == nil; entry, err = doc.Next() {
		if msoleps.IsMSOLEPS(entry.Initial) {
			if oerr := props.Reset(doc); oerr != nil {
				log.Printf("docparser: could not reset props: %v", oerr)
				err = oerr
				break
			}

			for _, prop := range props.Property {
				// log.Printf("Prop: %v | %v", prop.Name, prop.Type())
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
						m.Created = d.Time()
					case "LastSaveTime":
						m.Modified = d.Time()
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
		cmd.Stdin = bytes.NewBuffer(*d.data)
		cmd.Stdout = w
		err := cmd.Run()
		if err != nil {
			log.Printf("docparser: %v", err)
			if exitErr, ok := err.(*exec.ExitError); ok {
				log.Printf("%s", exitErr.Stderr)
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
		log.Printf("docparser: %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("%s", exitErr.Stderr)
		}
	}
	return string(out)
}
