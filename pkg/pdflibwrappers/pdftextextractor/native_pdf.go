package pdftextextractor

import (
	"bytes"
	"io"
	"strconv"

	"github.com/johbar/pdfcpu-lite/pkg/pdfcpu"
	"github.com/johbar/pdfcpu-lite/pkg/pdfcpu/model"
	"github.com/johbar/pdfcpu-lite/pkg/pdfcpu/validate"
	"github.com/johbar/text-extraction-service/v4/internal/cache"
	"github.com/johbar/text-extraction-service/v4/internal/pdfdateparser"
)

var pdfcpuConfig *model.Configuration = model.NewDefaultConfiguration()

type Document struct {
	ctx   model.Context
	path  string
	data  *[]byte
	pages int
}

func init() {
	pdfcpuConfig.Offline = true
	pdfcpuConfig.ValidateLinks = false
	pdfcpuConfig.ValidationMode = model.ValidationRelaxed
}

func Load(data []byte) (*Document, error) {
	rs := bytes.NewReader(data)
	ctx, err := pdfcpu.Read(rs, pdfcpuConfig)
	if err != nil {
		return nil, err
	}

	// api.ReadAndValidate() doesn't handle validation issues gracefully.
	// But the validation step seems to be necessary for fully initializing ctx.
	// So we use lower-level APIs instead but ignore validation errors.
	_ = validate.XRefTable(ctx)
	// necessary for image extraction
	_ = pdfcpu.OptimizeXRefTable(ctx)

	return &Document{ctx: *ctx, data: &data, pages: ctx.PageCount}, nil
}

func Open(path string) (*Document, error) {
	ctx, err := pdfcpu.ReadFile(path, pdfcpuConfig)
	if err != nil {
		return nil, err
	}

	// ignore validation error
	_ = validate.XRefTable(ctx)
	// necessary for image extraction
	_ = pdfcpu.OptimizeXRefTable(ctx)
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
	imgs, _ := pdfcpu.ExtractPageImages(&d.ctx, i+1, true)
	for x, img := range imgs {
		if img.Thumb {
			// ignore thumbnail images
			delete(imgs, x)
		}
	}
	text, _ := extractPageText(&d.ctx, i+1)
	var str string
	if text != nil && text.Len() > 0 {
		str = text.String()
	}
	return str, len(imgs) > 0
}

func (d *Document) StreamText(w io.Writer) error {
	for i := range d.Pages() {
		text, err := extractPageText(&d.ctx, i+1)
		if err != nil {
			continue
		}
		if text == nil {
			continue
		}
		_, err = text.WriteTo(w)
		if err != nil {
			return err
		}
	}
	return nil
}
