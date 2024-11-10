/*
Package puregopoppler loads the GLib interface library of the Poppler PDF library.
It exposes only some basic functions needed for extracting text and metadata from documents.
*/
package puregopoppler

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
	"golang.org/x/sys/unix"
)

var (
	free           func(unsafe.Pointer)
	g_bytes_new    func(bytes unsafe.Pointer, length uint64) uintptr
	g_bytes_unref  func(uintptr)
	g_object_unref func(uintptr)

	poppler_get_version             func() string
	poppler_document_new_from_bytes func(gbytes uintptr, password uintptr, error unsafe.Pointer) uintptr

	poppler_document_get_n_pages func(uintptr) int
	poppler_document_get_page    func(uintptr, int) uintptr

	poppler_document_get_pdf_version_string func(uintptr) *byte
	poppler_document_get_title              func(uintptr) *byte
	poppler_document_get_author             func(uintptr) *byte
	poppler_document_get_subject            func(uintptr) *byte
	poppler_document_get_keywords           func(uintptr) *byte
	poppler_document_get_creator            func(uintptr) *byte
	poppler_document_get_producer           func(uintptr) *byte
	poppler_document_get_metadata           func(uintptr) *byte
	poppler_document_get_creation_date      func(uintptr) int64
	poppler_document_get_modification_date  func(uintptr) int64

	poppler_page_get_text func(uintptr) *byte
	libnames              = []string{"libpoppler-glib.so", "libpoppler-glib.so.8"}
)

func init() {
	var lib uintptr
	var err error
	for _, libname := range libnames {
		lib, err = purego.Dlopen(libname, purego.RTLD_LAZY)
		if lib != 0 {
			break
		}
	}
	if err != nil {
		panic("Could not load libpoppler-glib")
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
	purego.RegisterLibFunc(&poppler_document_get_metadata, lib, "poppler_document_get_metadata")
	purego.RegisterLibFunc(&poppler_document_get_creation_date, lib, "poppler_document_get_creation_date")
	purego.RegisterLibFunc(&poppler_document_get_modification_date, lib, "poppler_document_get_modification_date")
}

// Version returns the version of the Poppler shared library
func Version() string {
	return poppler_get_version()
}

// Load opens a PDF from a byte slice
func Load(data []byte) (*Document, error) {
	ptr := unsafe.Pointer(&data[0])
	gbytes := g_bytes_new(ptr, uint64(len(data)))
	var err *GError
	doc := poppler_document_new_from_bytes(gbytes, 0, unsafe.Pointer(&err))
	if doc == 0 {
		return nil, errors.New("poppler: " + toStr(err.message))
	}
	g_bytes_unref(gbytes)
	d := &Document{doc}
	runtime.SetFinalizer(d, closeDoc)
	return d, nil
}

func closeDoc(d *Document) {
	if d != nil && d.uintptr != 0 {
		d.Close()
	}
	runtime.SetFinalizer(d, nil)
}

func closePage(p *Page) {
	if p != nil && p.uintptr != 0 {
		p.Close()
	}
	runtime.SetFinalizer(p, nil)
}

// toStr converts a C byte/char* pointer to a Go string and frees the memory allocation
func toStr(stringPtr *byte) string {
	str := unix.BytePtrToString(stringPtr)
	free(unsafe.Pointer(stringPtr))
	return str
}
