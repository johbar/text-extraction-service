/*
Package poppler_purego loads the GLib interface library of the Poppler PDF library.
It exposes only some basic functions needed for extracting text and metadata from documents.
*/
package poppler_purego

import (
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ebitengine/purego"
	"github.com/johbar/text-extraction-service/v2/internal/unix"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers"
)

type GError struct {
	_       uint32
	_       int32
	message *byte
}

// Page represents a PDF page opened by Poppler
type Page uintptr

type doc uintptr

// Document represents a PDF opened by Poppler
type Document struct {
	handle doc
	path   string
	data   *[]byte
	pages  int
}

var (
	lib            uintptr
	free           func(*byte)
	g_bytes_new    func(bytes []byte, length uint64) uintptr
	g_bytes_unref  func(uintptr)
	g_object_unref func(uintptr)

	poppler_get_version             func() string
	poppler_document_new_from_bytes func(gbytes uintptr, password uintptr, err uintptr) doc
	poppler_document_new_from_file  func(uri *byte, password uintptr, err uintptr) doc

	poppler_document_get_n_pages func(doc) int
	poppler_document_get_page    func(doc, int) Page

	poppler_document_get_pdf_version_string func(doc) *byte
	poppler_document_get_title              func(doc) *byte
	poppler_document_get_author             func(doc) *byte
	poppler_document_get_subject            func(doc) *byte
	poppler_document_get_keywords           func(doc) *byte
	poppler_document_get_creator            func(doc) *byte
	poppler_document_get_producer           func(doc) *byte
	// poppler_document_get_metadata           func(uintptr) *byte
	poppler_document_get_creation_date     func(doc) int64
	poppler_document_get_modification_date func(doc) int64
	// image related
	g_list_length                   func(glist uintptr) uint32
	poppler_page_get_image_mapping  func(page Page) uintptr
	poppler_page_free_image_mapping func(glist uintptr)

	poppler_page_get_text func(Page) *byte
	defaultLibNames       = []string{"libpoppler-glib.so", "libpoppler-glib.so.8", "/opt/homebrew/lib/libpoppler-glib.8.dylib", "/opt/homebrew/lib/libpoppler-glib.dylib", "libpoppler-glib.8.dylib"}
)

func InitLib(path string) (string, error) {
	var err error
	if len(path) > 0 {
		lib, path, err = pdflibwrappers.TryLoadLib(path)
	} else {
		lib, path, err = pdflibwrappers.TryLoadLib(defaultLibNames...)
	}

	if err != nil {
		return "", err
	}
	purego.RegisterLibFunc(&free, lib, "free")
	purego.RegisterLibFunc(&g_bytes_new, lib, "g_bytes_new")
	purego.RegisterLibFunc(&g_bytes_unref, lib, "g_bytes_unref")
	purego.RegisterLibFunc(&g_object_unref, lib, "g_object_unref")

	purego.RegisterLibFunc(&poppler_get_version, lib, "poppler_get_version")
	purego.RegisterLibFunc(&poppler_document_new_from_bytes, lib, "poppler_document_new_from_bytes")
	purego.RegisterLibFunc(&poppler_document_new_from_file, lib, "poppler_document_new_from_file")
	purego.RegisterLibFunc(&poppler_document_get_n_pages, lib, "poppler_document_get_n_pages")
	purego.RegisterLibFunc(&poppler_document_get_page, lib, "poppler_document_get_page")
	purego.RegisterLibFunc(&poppler_page_get_text, lib, "poppler_page_get_text")
	purego.RegisterLibFunc(&poppler_document_get_pdf_version_string, lib, "poppler_document_get_pdf_version_string")
	purego.RegisterLibFunc(&poppler_document_get_title, lib, "poppler_document_get_title")
	purego.RegisterLibFunc(&poppler_document_get_author, lib, "poppler_document_get_author")
	purego.RegisterLibFunc(&poppler_document_get_subject, lib, "poppler_document_get_subject")
	purego.RegisterLibFunc(&poppler_document_get_keywords, lib, "poppler_document_get_keywords")
	purego.RegisterLibFunc(&poppler_document_get_creator, lib, "poppler_document_get_creator")
	purego.RegisterLibFunc(&poppler_document_get_producer, lib, "poppler_document_get_producer")
	// purego.RegisterLibFunc(&poppler_document_get_metadata, lib, "poppler_document_get_metadata")
	purego.RegisterLibFunc(&poppler_document_get_creation_date, lib, "poppler_document_get_creation_date")
	purego.RegisterLibFunc(&poppler_document_get_modification_date, lib, "poppler_document_get_modification_date")
	purego.RegisterLibFunc(&g_list_length, lib, "g_list_length")
	purego.RegisterLibFunc(&poppler_page_get_image_mapping, lib, "poppler_page_get_image_mapping")
	purego.RegisterLibFunc(&poppler_page_free_image_mapping, lib, "poppler_page_free_image_mapping")

	return path, nil
}

// Version returns the version of the Poppler shared library
func Version() string {
	return poppler_get_version()
}

// Load opens a PDF from a byte slice
func Load(data []byte) (*Document, error) {
	gbytes := g_bytes_new(data, uint64(len(data)))
	defer g_bytes_unref(gbytes)
	handle := poppler_document_new_from_bytes(gbytes, 0, 0)
	if handle == 0 {
		return nil, errors.New("poppler: could not load PDF")
	}
	d := &Document{handle: handle, data: &data, pages: poppler_document_get_n_pages(handle)}
	return d, nil
}

func Open(path string) (*Document, error) {
	// Poppler needs a file URI which does not support relative local paths
	abPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	ptr, err := unix.BytePtrFromString("file:" + abPath)
	if err != nil {
		return nil, err
	}
	handle := poppler_document_new_from_file(ptr, 0, 0)
	if handle == 0 {
		return nil, errors.New("poppler: could not load PDF at " + path)
	}
	d := &Document{handle: handle, path: path, pages: poppler_document_get_n_pages(handle)}
	return d, nil
}

// toStr converts a C byte/char* pointer to a Go string and frees the memory allocated
func toStr(stringPtr *byte) string {
	str := unix.BytePtrToString(stringPtr)
	free(stringPtr)
	return str
}

func (d *Document) Data() *[]byte {
	return d.data
}

func (d *Document) Pages() int {
	return d.pages
}

func (d *Document) Path() string {
	return d.path
}

// GetPage opens a PDF page by index (zero-based)
func (d *Document) GetPage(i int) Page {
	p := poppler_document_get_page(d.handle, i)
	return p
}

func (d *Document) Close() {
	g_object_unref((uintptr(d.handle)))
	d.handle = 0
}

func (d *Document) Text(pageIndex int) (string, bool) {
	p := d.GetPage(pageIndex)
	txt := p.Text()
	var images uint32 = 0
	if len(txt) == 0 {
		images = p.countImages()
	}
	p.Close()
	return txt, images > 0
}

// StreamText writes the document's plain text content to an io.Writer
func (d *Document) StreamText(w io.Writer) error {
	for n := 0; n < d.pages; n++ {
		page := d.GetPage(n)
		pageText := page.Text()
		_, err := w.Write([]byte(pageText))
		if err != nil {
			page.Close()
			return err
		}
		page.Close()
	}
	return nil
}

// Metadata returns some of the PDF metadata as map with keys compatible to HTTP headers
func (d *Document) MetadataMap() map[string]string {
	m := make(map[string]string)
	m["x-parsed-by"] = "Poppler"
	m["x-doctype"] = "pdf"

	if val := poppler_document_get_n_pages(d.handle); val != 0 {
		m["x-document-pages"] = strconv.Itoa(val)
	}
	if val := toStr(poppler_document_get_pdf_version_string(d.handle)); val != "" {
		m["x-document-version"] = val
	}
	if val := toStr(poppler_document_get_author(d.handle)); val != "" {
		m["x-document-author"] = val
	}
	if val := toStr(poppler_document_get_title(d.handle)); val != "" {
		m["x-document-title"] = val
	}
	if val := toStr(poppler_document_get_subject(d.handle)); val != "" {
		m["x-document-subject"] = val
	}
	if val := toStr(poppler_document_get_keywords(d.handle)); val != "" {
		m["x-document-keywords"] = val
	}
	if val := toStr(poppler_document_get_creator(d.handle)); len(val) > 0 {
		m["x-document-creator"] = val
	}
	if val := toStr(poppler_document_get_producer(d.handle)); len(val) > 0 {
		m["x-document-producer"] = val
	}
	if val := poppler_document_get_creation_date(d.handle); val != 0 {
		createTime := time.Unix(val, 0)
		m["x-document-created"] = createTime.Format(time.RFC3339)
	}
	if val := poppler_document_get_modification_date(d.handle); val != 0 {
		modTime := time.Unix(val, 0)
		m["x-document-modified"] = modTime.Format(time.RFC3339)
	}

	return m
}

// Text returns the pages textual content
func (p Page) Text() string {
	txtPtr := poppler_page_get_text(p)
	return toStr(txtPtr)
}

// countImages returns the number of images on page
func (p Page) countImages() uint32 {
	list := poppler_page_get_image_mapping(p)
	defer poppler_page_free_image_mapping(list)
	length := g_list_length(list)
	return length
}

// Close closes the page, freeing up resources allocated when the page was opened
func (p Page) Close() {
	if p != 0 {
		g_object_unref(uintptr(p))
	}
	// do nothing if null pointer
}
