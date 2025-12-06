package officexmlparser

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/klauspost/compress/zip"
)

type XmlBasedDocument struct {
	data         *[]byte
	contentFiles []*zip.File
	ext          string
	bodyTag      string
	metadata     map[string]string
	path         string
	file         *os.File
}

var (
	contentFileNames = []string{"content.xml", "word/document.xml"}

	ErrContentNotFound  = errors.New("content file not found in ZIP file")
	ErrReadingZipFailed = errors.New("could not read ZIP file")
)

var breaks = []string{"p", "h", "br"}

func Open(path string, ext string) (*XmlBasedDocument, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	d, err := New(f, info.Size(), ext)
	d.path = path
	d.file = f
	return d, err
}

func NewFromBytes(data []byte, ext string) (*XmlBasedDocument, error) {
	d, err := New(bytes.NewReader(data), int64(len(data)), ext)
	if err != nil {
		return nil, err
	}
	d.data = &data
	return d, nil
}

// New opens the zip file and extracts the relevant files containing text content
func New(r io.ReaderAt, size int64, ext string) (*XmlBasedDocument, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	md := make(map[string]string)
	md["x-parsed-by"] = "text-extraction-service"
	md["x-doctype"] = ext
	d := &XmlBasedDocument{ext: ext, bodyTag: "body", metadata: md}
	for _, f := range zr.File {
		if slices.Contains(contentFileNames, f.Name) {
			d.contentFiles = append(d.contentFiles, f)
		}
		// Open Office Document
		if f.Name == "meta.xml" {
			if data, err := readFileFromZip(f); err == nil {
				mapOpenDocumentMetadata(d.metadata, data)
			}
		}
		// MS Office
		if f.Name == "docProps/app.xml" {
			if data, err := readFileFromZip(f); err == nil {
				mapMsOfficeStats(d.metadata, data)
			}
		}
		if f.Name == "docProps/core.xml" {
			if data, err := readFileFromZip(f); err == nil {
				mapMsOfficeCoreMetadata(d.metadata, data)
			}
		}

		if strings.HasPrefix(f.Name, "ppt/slides/") && strings.HasSuffix(f.Name, ".xml") {
			// This is a powerpoint file. We need to add all slides.
			d.contentFiles = append(d.contentFiles, f)
			// Their structure is different.
			d.bodyTag = "cSld"
		}
	}
	if len(d.contentFiles) == 0 {
		return nil, ErrContentNotFound
	}
	return d, nil
}

func readFileFromZip(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	buf := make([]byte, f.UncompressedSize64)
	_, err = r.Read(buf)
	if err == io.EOF {
		return buf, nil
	}
	return buf, nil
}

func mapMsOfficeCoreMetadata(metadata map[string]string, data []byte) {
	var msMeta MsOfficeCoreMetadata
	if err := xml.Unmarshal(data, &msMeta); err == nil {
		if len(msMeta.Creator) > 0 {
			metadata["x-document-creator"] = msMeta.Creator
		}
		if len(msMeta.Publisher) > 0 {
			metadata["x-document-author"] = msMeta.Publisher
		}
		if len(msMeta.Title) > 0 {
			metadata["x-document-title"] = msMeta.Title
		}
		if len(msMeta.Subject) > 0 {
			metadata["x-document-subject"] = msMeta.Subject
		}
		if len(msMeta.Keywords) > 0 {
			metadata["x-document-keywords"] = msMeta.Keywords
		}
		if len(msMeta.Created) > 0 {
			metadata["x-document-created"] = msMeta.Created
		}
		if len(msMeta.Modified) > 0 {
			metadata["x-document-modified"] = msMeta.Modified
		}
		// Do we want this?
		// metadata["x-document-description"] = msMeta.Description
	}
}

func mapMsOfficeStats(metadata map[string]string, data []byte) {
	var stats MsOfficeStats
	if err := xml.Unmarshal(data, &stats); err == nil {
		if len(stats.Generator) > 0 {
			metadata["x-document-producer"] = stats.Generator
		}
		if len(stats.PageCount) > 0 {
			metadata["x-document-pages"] = stats.PageCount
		}
		if len(stats.WordCount) > 0 {
			metadata["x-document-words"] = stats.WordCount
		}
		if len(stats.CharCount) > 0 {
			metadata["x-document-chars"] = stats.CharCount
		}
		if len(stats.ParagraphCount) > 0 {
			metadata["x-document-paragraphs"] = stats.ParagraphCount
		}
	}
}

func mapOpenDocumentMetadata(metadata map[string]string, data []byte) {
	var odMetaContainer OpenDocumentMetadata
	if err := xml.Unmarshal(data, &odMetaContainer); err == nil {
		odMeta := odMetaContainer.Meta
		if len(odMeta.CreationDate) > 0 {
			metadata["x-document-created"] = odMeta.CreationDate
		}
		if len(odMeta.Generator) > 0 {
			metadata["x-document-producer"] = odMeta.Generator
		}
		if len(odMeta.Creator) > 0 {
			metadata["x-document-creator"] = odMeta.Creator
		}
		if len(odMeta.Title) > 0 {
			metadata["x-document-title"] = odMeta.Title
		}
		// Do we want this?
		// d.metadata["x-document-description"] = odMeta.Description
		if len(odMeta.Publisher) > 0 {
			metadata["x-document-author"] = odMeta.Publisher
		}
		if len(odMeta.Subject) > 0 {
			metadata["x-document-subject"] = odMeta.Subject
		}
		if len(odMeta.Date) > 0 {
			metadata["x-document-modified"] = odMeta.Date
		}
		if len(odMeta.Keywords) > 0 {
			metadata["x-document-keywords"] = strings.Join(odMeta.Keywords, " ")
		}
		if len(odMeta.Stats.PageCount) > 0 {
			metadata["x-document-pages"] = odMeta.Stats.PageCount
		}
		if len(odMeta.Stats.WordCount) > 0 {
			metadata["x-document-words"] = odMeta.Stats.WordCount
		}
		if len(odMeta.Stats.CharCount) > 0 {
			metadata["x-document-chars"] = odMeta.Stats.CharCount
		}
		if len(odMeta.Stats.ParagraphCount) > 0 {
			metadata["x-document-paragraphs"] = odMeta.Stats.ParagraphCount
		}
	}
}

func (d *XmlBasedDocument) StreamText(w io.Writer) error {
	var errs error
	for _, f := range d.contentFiles {
		r, err := f.Open()
		errs = errors.Join(errs, err)
		// we don't want to abort when processing one of the contentFiles fails,
		// but still inform the caller of all errors
		if err != nil {
			continue
		}
		errs = errors.Join(errs, XmlToText(r, w, d.bodyTag, breaks))
		r.Close()
	}
	return errs
}

func (d *XmlBasedDocument) Pages() int {
	// it is not possible to query these docs page per page
	// so by convention we return -1
	return -1
}

func (d *XmlBasedDocument) Path() string {
	return d.path
}

func (d *XmlBasedDocument) Data() *[]byte {
	return d.data
}

func (d *XmlBasedDocument) MetadataMap() map[string]string {
	return d.metadata
}

func (d *XmlBasedDocument) Text(_ int) (string, bool) {
	panic("not allowed")
}

func (d *XmlBasedDocument) HasNewlines() bool {
	return true
}

func (d *XmlBasedDocument) Close() {
	if d.file != nil {
		d.file.Close()
	}
	d.data = nil
}
