package pdftextextractor

import (
	"bytes"
	"io"
	"os"
	"strconv"

	"github.com/johbar/text-extraction-service/v4/internal/cache"
	"github.com/johbar/text-extraction-service/v4/internal/pdfdateparser"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type Document struct {
	ctx   model.Context
	path  string
	data  *[]byte
	pages int
}

func Load(data []byte) (*Document, error) {
	rs := bytes.NewReader(data)
	api.DisableConfigDir()
	ctx, err := api.ReadAndValidate(rs, api.LoadConfiguration())
	if err != nil {
		return nil, err
	}
	return &Document{ctx: *ctx, data: &data, pages: ctx.PageCount}, nil
}

func Open(path string) (*Document, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	api.DisableConfigDir()
	ctx, err := api.ReadAndValidate(f, api.LoadConfiguration())
	if err != nil {
		return nil, err
	}
	return &Document{ctx: *ctx, path: path, pages: ctx.PageCount}, nil
}

func (d *Document) Pages() int {
	return d.pages
}

func (d *Document) Path() string {
	return d.path
}

func (d *Document) Data() *[]byte {
	return d.data
}

func (d *Document) HasNewlines() bool { return true }

func (d *Document) Close() {
	// noop
}

func (d *Document) MetadataMap() cache.DocumentMetadata {
	return map[string]string{
		"x-document-author":   d.ctx.Author,
		"x-document-creator":  d.ctx.Creator,
		"x-document-title":    d.ctx.Title,
		"x-document-subject":  d.ctx.Subject,
		"x-document-producer": d.ctx.Producer,
		"x-document-version":  "PDF-" + d.ctx.VersionString(),
		"x-document-keywords": d.ctx.Keywords,
		"x-document-pages":    strconv.Itoa(d.pages),
		"x-document-created":  pdfdateparser.PdfDateToIso(d.ctx.XRefTable.CreationDate),
		"x-document-modified": pdfdateparser.PdfDateToIso(d.ctx.ModDate),
		"x-parsed-by":         "text-extraction-service",
		"x-doc-type":          "pdf",
	}
}

func (d *Document) Text(i int) (string, bool) {
	text, err := extractPageText(&d.ctx, i+1)
	if err != nil {
		// FIXME
		panic(err)
	}
	return text.String(), false
}

func (d *Document) StreamText(w io.Writer) error {
	for i := range d.Pages() {
		text, err := extractPageText(&d.ctx, i+1)
		if err != nil {
			return err
		}
		_, err = text.WriteTo(w)
		if err != nil {
			return err
		}
	}
	return nil
}
