/*
Package poppler_purego loads the GLib interface library of the Poppler PDF library.
It exposes only some basic functions needed for extracting text and metadata from documents.
*/
package poppler_purego

import (
	"errors"
	"io"
	"strconv"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/johbar/text-extraction-service/v2/internal/unix"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers"
)

type DocumentInfo struct {
	PdfVersion, Title, Author, Subject, KeyWords, Creator, Producer, Metadata string
	CreationDate, ModificationDate                                            int64
	Pages                                                                     int
}

type GError struct {
	_       uint32
	_       int32
	message *byte
}

// PopplerPage represents a PDF page opened by Poppler
type PopplerPage struct {
	uintptr
}

// Document represents a PDF opened by Poppler
type Document struct {
	handle uintptr
	data   *[]byte
	pages  int
}

var (
	free           func(unsafe.Pointer)
	g_bytes_new    func(bytes unsafe.Pointer, length uint64) uintptr
	g_bytes_unref  func(uintptr)
	g_object_unref func(uintptr)

	poppler_get_version             func() string
	poppler_document_new_from_bytes func(gbytes uintptr, password uintptr, err unsafe.Pointer) uintptr

	poppler_document_get_n_pages func(uintptr) int
	poppler_document_get_page    func(uintptr, int) uintptr

	poppler_document_get_pdf_version_string func(uintptr) *byte
	poppler_document_get_title              func(uintptr) *byte
	poppler_document_get_author             func(uintptr) *byte
	poppler_document_get_subject            func(uintptr) *byte
	poppler_document_get_keywords           func(uintptr) *byte
	poppler_document_get_creator            func(uintptr) *byte
	poppler_document_get_producer           func(uintptr) *byte
	// poppler_document_get_metadata           func(uintptr) *byte
	poppler_document_get_creation_date     func(uintptr) int64
	poppler_document_get_modification_date func(uintptr) int64

	poppler_page_get_text func(uintptr) *byte
	defaultLibNames       = []string{"libpoppler-glib.so", "libpoppler-glib.so.8", "/opt/homebrew/lib/libpoppler-glib.8.dylib", "/opt/homebrew/lib/libpoppler-glib.dylib", "libpoppler-glib.8.dylib"}
)

func InitLib(path string) (string, error) {
	var lib uintptr
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

	return path, nil
}

// Version returns the version of the Poppler shared library
func Version() string {
	return poppler_get_version()
}

// Load opens a PDF from a byte slice
func Load(data []byte) (*Document, error) {
	ptr := unsafe.Pointer(&data[0])
	gbytes := g_bytes_new(ptr, uint64(len(data)))
	defer g_bytes_unref(gbytes)
	var err *GError
	handle := poppler_document_new_from_bytes(gbytes, 0, unsafe.Pointer(&err))
	if handle == 0 {
		return nil, errors.New("poppler: " + toStr(err.message))
	}
	d := &Document{handle: handle, data: &data, pages: poppler_document_get_n_pages(handle)}
	return d, nil
}

// toStr converts a C byte/char* pointer to a Go string and frees the memory allocated
func toStr(stringPtr *byte) string {
	str := unix.BytePtrToString(stringPtr)
	free(unsafe.Pointer(stringPtr))
	return str
}

func (d *Document) Data() *[]byte {
	return d.data
}

func (d *Document) Pages() int {
	return d.pages
}

// GetPage opens a PDF page by index (zero-based)
func (d *Document) GetPage(i int) *PopplerPage {
	p := &PopplerPage{poppler_document_get_page(d.handle, i)}
	return p
}

func (d *Document) Close() {
	g_object_unref(d.handle)
	d.handle = 0
}

// Text returns the pages textual content
func (p *PopplerPage) Text() string {
	txtPtr := poppler_page_get_text(p.uintptr)
	return toStr(txtPtr)
}

// Close closes the page, freeing up resources allocated when the page was opened
func (p *PopplerPage) Close() {
	if p.uintptr != 0 {
		g_object_unref(p.uintptr)
		p.uintptr = 0
	}
	// do nothing if null pointer
}

func (d *Document) Text(pageIndex int) (string, bool) {
	page := poppler_document_get_page(d.handle, pageIndex)
	txtPtr := poppler_page_get_text(page)
	g_object_unref(page)
	// FIXME: implement image discovery logic to be true about hasImages
	return toStr(txtPtr), true
}

// StreamText writes the document's plain text content to an io.Writer
func (d *Document) StreamText(w io.Writer) error {
	// logger.Debug("Extracting", "pages", d.GetNPages())
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
