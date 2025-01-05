// Package pdfium_purego loads the mupdf shard library and exposes some functions needed for text extraction.
package pdfium_purego

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/johbar/text-extraction-service/v2/internal/pdfdateparser"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var (
	FPDF_InitLibrary    func()
	FPDF_DestroyLibrary func()

	FPDF_LoadMemDocument func(data unsafe.Pointer, length uint64, password unsafe.Pointer) uintptr
	// document
	FPDF_GetPageCount  func(docHandle uintptr) int
	FPDF_CloseDocument func(docHandle uintptr)
	//Get the file version of the specific PDF document.
	FPDF_GetFileVersion func(doc uintptr, version unsafe.Pointer) bool

	// page
	FPDF_LoadPage  func(docHandle uintptr, index int32) uintptr
	FPDF_ClosePage func(pageHandle uintptr)
	// text
	FPDFText_LoadPage   func(pageHandle uintptr) uintptr
	FPDFText_ClosePage  func(textHandle uintptr)
	FPDFText_CountChars func(textHandle uintptr) int

	// Extract unicode text string from the page, in UTF-16LE encoding.
	// Returns Number of characters written into parameter result buffer, excluding the trailing terminator.
	FPDFText_GetText    func(textHandle uintptr, startIndex int, count int, resultBuf unsafe.Pointer) (charsWritten int)
	FPDFText_GetUnicode func(textHandle uintptr, index int) rune
	/*
			Get the text string of specific tag from meta data of a PDF document.

		Regardless of the platform, the text string is alway in UTF-16LE encoding.
		That means the buffer can be treated as an array of WORD (on Intel and compatible CPUs),
		each WORD representing the Unicode of a character(some special Unicode may take 2 WORDs).
		The string is followed by two bytes of zero indicating the end of the string.
	*/
	FPDF_GetMetaText func(documenthandle uintptr, tag string, resultBuf unsafe.Pointer, bufLength uint64) (bytesNeeded uint64)

	// PDFium is not thread-safe. This lock guards the lib against concurrent access in places where this is known to be necessary
	Lock            sync.Mutex
	defaultLibNames = []string{"libpdfium.so", "/usr/lib/libreoffice/program/libpdfiumlo.so", "libpdfium.dylib"}
)

type Document struct {
	handle uintptr
	data   []byte
	pages  int
}

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
	purego.RegisterLibFunc(&FPDF_InitLibrary, lib, "FPDF_InitLibrary")
	purego.RegisterLibFunc(&FPDF_DestroyLibrary, lib, "FPDF_DestroyLibrary")
	purego.RegisterLibFunc(&FPDF_LoadMemDocument, lib, "FPDF_LoadMemDocument")
	purego.RegisterLibFunc(&FPDF_CloseDocument, lib, "FPDF_CloseDocument")
	purego.RegisterLibFunc(&FPDF_GetFileVersion, lib, "FPDF_GetFileVersion")
	purego.RegisterLibFunc(&FPDF_GetPageCount, lib, "FPDF_GetPageCount")
	purego.RegisterLibFunc(&FPDF_LoadPage, lib, "FPDF_LoadPage")
	purego.RegisterLibFunc(&FPDF_ClosePage, lib, "FPDF_ClosePage")
	purego.RegisterLibFunc(&FPDFText_LoadPage, lib, "FPDFText_LoadPage")
	purego.RegisterLibFunc(&FPDFText_ClosePage, lib, "FPDFText_ClosePage")
	purego.RegisterLibFunc(&FPDFText_CountChars, lib, "FPDFText_CountChars")
	purego.RegisterLibFunc(&FPDFText_GetText, lib, "FPDFText_GetText")
	purego.RegisterLibFunc(&FPDFText_GetUnicode, lib, "FPDFText_GetUnicode")
	purego.RegisterLibFunc(&FPDF_GetMetaText, lib, "FPDF_GetMetaText")

	FPDF_InitLibrary()
	return path, nil
}

func Load(data []byte) (*Document, error) {
	handle := FPDF_LoadMemDocument(unsafe.Pointer(&(data[0])), uint64(len(data)), nil)
	if handle == 0 {
		return nil, errors.New("pdfium: cannot load document")
	}
	return &Document{data: data, handle: handle, pages: FPDF_GetPageCount(handle)}, nil
}

func (d *Document) Close() {
	if d.handle != 0 {
		FPDF_CloseDocument(d.handle)
		d.handle = 0
	}
	d.data = nil
}

// Text returns page i's text
func (d *Document) Text(i int) string {
	pageHandle := FPDF_LoadPage(d.handle, int32(i))
	defer FPDF_ClosePage(pageHandle)
	pageTextHandle := FPDFText_LoadPage(pageHandle)
	defer FPDFText_ClosePage(pageTextHandle)
	charCount := FPDFText_CountChars(pageTextHandle)
	charData := make([]byte, (charCount+1)*2)
	charsWritten := FPDFText_GetText(pageTextHandle, 0, charCount, unsafe.Pointer(&charData[0]))
	// strip 2 trailing NUL bytes
	result, _ := transformUtf16LeToUtf8(charData[0 : 2*charsWritten])
	// PDFium inserts NUL bytes
	result = strings.ReplaceAll(result, "\x00", "\n")
	// PDFium replaces hyphens with `noncharacters`:
	result = strings.ReplaceAll(result, "\uFFFE", "")
	return result
}

func (d *Document) StreamText(w io.Writer) {
	for i := range d.pages {
		pageText := d.Text(i)
		w.Write([]byte(pageText))
	}
}

func (d *Document) ProcessPages(w io.Writer, process func(pageText string, pageIndex int, w io.Writer, pdfData *[]byte)) {
	for i := range d.pages {
		process(d.Text(i), i, w, &d.data)
	}
}

func (d *Document) MetadataMap() map[string]string {
	m := make(map[string]string)
	m["x-parsed-by"] = "PDFium"
	m["x-doctype"] = "pdf"
	m["x-document-pages"] = strconv.Itoa(d.pages)

	var version int
	if ok := FPDF_GetFileVersion(d.handle, unsafe.Pointer(&version)); ok {
		// the result is 18 for version 1.8 etc
		m["x-document-version"] = fmt.Sprintf("PDF-%.1f", float32(version)/10)
	}

	// we use the same (oversized) byte array for all fields
	var defaultSize uint64 = 512
	buf := make([]byte, defaultSize)
	lookup := func(key string) (ok bool, value string) {
		requiredSize := FPDF_GetMetaText(d.handle, key, unsafe.Pointer(&buf[0]), uint64(len(buf)))
		if requiredSize <= 2 {
			return false, ""
		}
		if requiredSize > uint64(len(buf)) {
			// if the buffer was too small, allocate a bigger one
			buf = make([]byte, requiredSize)
			FPDF_GetMetaText(d.handle, key, unsafe.Pointer(&buf[0]), uint64(len(buf)))
		}
		// Strip the last two bytes (NULs)
		str, _ := transformUtf16LeToUtf8(buf[0 : requiredSize-2])
		return true, str
	}

	if ok, val := lookup("Title"); ok {
		m["x-document-title"] = val
	}
	if ok, val := lookup("Author"); ok {
		m["x-document-author"] = val
	}
	if ok, val := lookup("Subject"); ok {
		m["x-document-subject"] = val
	}
	if ok, val := lookup("Keywords"); ok {
		m["x-document-keywords"] = val
	}
	if ok, val := lookup("Creator"); ok {
		m["x-document-creator"] = val
	}
	if ok, val := lookup("Producer"); ok {
		m["x-document-producer"] = val
	}
	if ok, val := lookup("CreationDate"); ok {
		m["x-document-created"] = pdfdateparser.PdfDateToIso(val)
	}
	if ok, val := lookup("ModDate"); ok {
		m["x-document-modified"] = pdfdateparser.PdfDateToIso(val)
	}
	return m
}

func transformUtf16LeToUtf8(charData []byte) (string, error) {
	result, _, err := transform.Bytes(unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder(), charData)
	if err != nil {
		return "", err
	}
	return string(result), nil
}
