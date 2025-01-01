// Package mupdf_purego loads the mupdf shard library and exposes some functions needed for text extraction.
package mupdf_purego

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/johbar/text-extraction-service/v2/internal/pdfdateparser"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers"
	"golang.org/x/sys/unix"
)

// Document represents fitz document.
type Document struct {
	ctx    fzContext
	data   *[]byte
	doc    fzDocument
	mtx    sync.Mutex
	stream fzStream
	pages  int
}

type fzContext uintptr
type fzDocument uintptr
type fzStream uintptr
type fzBuffer uintptr
type fzSTextOptions uintptr

const (
	fzMaxStore uint64 = (256 << 20)
)

var (
	MuPdfVersion = "1.25.2"

	fz_new_context_imp func(alloc uintptr, locks uintptr, maxStore uint64, version string) fzContext
	fz_drop_context    func(ctx fzContext)
	/**
	Open a document using the specified stream object rather than
	opening a file on disk.

	magic: a string used to detect document type; either a file name
	or mime-type.

	stream: a stream representing the contents of the document file.

	NOTE: The caller retains ownership of 'stream' - the document will take its
	own reference if required.
	*/
	fz_open_document_with_stream   func(ctx fzContext, magic string, stream fzStream) fzDocument
	fz_drop_document               func(ctx fzContext, doc fzDocument)
	fz_drop_stream                 func(ctx fzContext, stream fzStream)
	fz_open_memory                 func(ctx fzContext, data unsafe.Pointer, len uint64) fzStream
	fz_register_document_handlers  func(ctx fzContext)
	fz_new_buffer_from_page_number func(ctx fzContext, doc fzDocument, number int, fzSTextOptions fzSTextOptions) fzBuffer
	fz_string_from_buffer          func(ctx fzContext, buf fzBuffer) string
	fz_drop_buffer                 func(ctx fzContext, buf fzBuffer)
	fz_count_pages                 func(ctx fzContext, doc fzDocument) int
	fz_lookup_metadata             func(ctx fzContext, doc fzDocument, key string, buf unsafe.Pointer, size int) int32

	defaultLibNames = []string{"libmupdf.so", "libmupdf.dylib", "/usr/local/lib/libmupdf.so"}
)

func InitLib(path string) error {
	var libmupdf uintptr
	var err error
	if len(path) > 0 {
		libmupdf, err = pdflibwrappers.TryLoadLib(path)
	} else {
		libmupdf, err = pdflibwrappers.TryLoadLib(defaultLibNames...)
	}

	if err != nil {
		return err
	}
	purego.RegisterLibFunc(&fz_new_context_imp, libmupdf, "fz_new_context_imp")
	purego.RegisterLibFunc(&fz_drop_context, libmupdf, "fz_drop_context")
	purego.RegisterLibFunc(&fz_drop_document, libmupdf, "fz_drop_document")
	purego.RegisterLibFunc(&fz_drop_stream, libmupdf, "fz_drop_stream")
	purego.RegisterLibFunc(&fz_open_document_with_stream, libmupdf, "fz_open_document_with_stream")
	purego.RegisterLibFunc(&fz_open_memory, libmupdf, "fz_open_memory")
	purego.RegisterLibFunc(&fz_register_document_handlers, libmupdf, "fz_register_document_handlers")
	purego.RegisterLibFunc(&fz_count_pages, libmupdf, "fz_count_pages")
	purego.RegisterLibFunc(&fz_new_buffer_from_page_number, libmupdf, "fz_new_buffer_from_page_number")
	purego.RegisterLibFunc(&fz_string_from_buffer, libmupdf, "fz_string_from_buffer")
	purego.RegisterLibFunc(&fz_lookup_metadata, libmupdf, "fz_lookup_metadata")
	purego.RegisterLibFunc(&fz_drop_buffer, libmupdf, "fz_drop_buffer")
	ver := version()
	if ver != "" {
		MuPdfVersion = ver
	}
	if ver := version(); ver != "" {
		MuPdfVersion = ver
	} else {
		return errors.New("cannot determine MuPDF version needed to create fz_context")
	}
	return nil
}

// Load returns new fitz document from byte slice.
func Load(b []byte) (f *Document, err error) {
	f = &Document{}

	f.ctx = fz_new_context_imp(0, 0, fzMaxStore, MuPdfVersion)
	if f.ctx == 0 {
		return nil, errors.New("mupdf: cannot create fitz context")
	}

	fz_register_document_handlers(f.ctx)

	f.stream = fz_open_memory(f.ctx, unsafe.Pointer(&b[0]), uint64(len(b)))
	if f.stream == 0 {
		fz_drop_context(f.ctx)
		return nil, errors.New("mupdf: cannot read memory buffer")
	}

	f.data = &b

	f.doc = fz_open_document_with_stream(f.ctx, "application/pdf", f.stream)
	if f.doc == 0 {
		fz_drop_stream(f.ctx, f.stream)
		fz_drop_context(f.ctx)
		return nil, errors.New("mupdf: cannot open document")
	}
	f.pages = fz_count_pages(f.ctx, f.doc)
	return
}

// Close closes the underlying fitz document.
func (f *Document) Close() {
	if f.stream != 0 {
		fz_drop_stream(f.ctx, f.stream)
	}
	fz_drop_document(f.ctx, f.doc)
	fz_drop_context(f.ctx)

	f.data = nil
}

// NumPages returns total number of pages in document.
func (d *Document) NumPages() int {
	return d.pages
}

// Text returns text for given page number.
func (d *Document) Text(pageIndex int) (string, error) {
	d.mtx.Lock()
	defer d.mtx.Unlock()
	buf := fz_new_buffer_from_page_number(d.ctx, d.doc, pageIndex, 0)
	if buf == 0 {
		return "", errors.New("cannot open page")
	}
	txt := fz_string_from_buffer(d.ctx, buf)
	fz_drop_buffer(d.ctx, buf)
	return txt, nil
}

func (d *Document) StreamText(w io.Writer) {
	for i := 0; i < d.NumPages(); i++ {
		txt, _ := d.Text(i)
		w.Write([]byte(txt))
	}
}
func (d *Document) ProcessPages(w io.Writer, process func(pageText string, pageIndex int, w io.Writer, pdfData *[]byte)) {
	for i := range d.pages {
		txt, _ := d.Text(i)
		process(txt, i, w, d.data)
	}
}

// Metadata returns a map with standard metadata.
func (d *Document) MetadataMap() map[string]string {
	m := make(map[string]string)
	m["x-parsed-by"] = "MuPDF"
	m["x-doctype"] = "pdf"
	buf := make([]byte, 256)

	lookup := func(key string) (ok bool, metadata string) {
		size := fz_lookup_metadata(d.ctx, d.doc, key, unsafe.Pointer(&buf[0]), len(buf))
		if size > int32(len(buf)) {
			buf = make([]byte, size)
			fz_lookup_metadata(d.ctx, d.doc, key, unsafe.Pointer(&buf[0]), len(buf))
		}
		return size >= 1, unix.ByteSliceToString(buf)
	}

	if ok, val := lookup("format"); ok {
		m["x-document-version"] = strings.Replace(val, " ", "-", 1)
	}
	if ok, val := lookup("info:Title"); ok {
		m["x-document-title"] = val
	}
	if ok, val := lookup("info:Author"); ok {
		m["x-document-author"] = val
	}
	if ok, val := lookup("info:Subject"); ok {
		m["x-document-subject"] = val
	}
	if ok, val := lookup("info:Keywords"); ok {
		m["x-document-keywords"] = val
	}
	if ok, val := lookup("info:Creator"); ok {
		m["x-document-creator"] = val
	}
	if ok, val := lookup("info:Producer"); ok {
		m["x-document-producer"] = val
	}
	if ok, val := lookup("info:CreationDate"); ok {
		m["x-document-created"] = pdfdateparser.PdfDateToIso(val)
	}
	if ok, val := lookup("info:ModDate"); ok {
		m["x-document-modified"] = pdfdateparser.PdfDateToIso(val)
	}
	pages := d.NumPages()
	m["x-document-pages"] = strconv.Itoa(pages)
	return m
}

// Taken from go-fitz
func version() string {
	if os.Getenv("MUPDF_VERSION") != "" {
		MuPdfVersion = os.Getenv("MUPDF_VERSION")
	}
	if ctx := fz_new_context_imp(0, 0, fzMaxStore, MuPdfVersion); ctx != 0 {
		fz_drop_context(ctx)
		return MuPdfVersion
	}

	s := strings.Split(MuPdfVersion, ".")
	v := strings.Join(s[:len(s)-1], ".")

	for x := 10; x >= 0; x-- {
		ver := v + "." + strconv.Itoa(x)
		if ver == MuPdfVersion {
			continue
		}

		if ctx := fz_new_context_imp(0, 0, fzMaxStore, ver); ctx != 0 {
			fz_drop_context(ctx)
			return ver
		}
	}

	return ""
}
