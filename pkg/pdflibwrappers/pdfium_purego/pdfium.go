// Package pdfium_purego loads the PDFium shard library and exposes some functions needed for text extraction.
package pdfium_purego

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/johbar/text-extraction-service/v4/internal/pdfdateparser"
	"github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type document uintptr
type page uintptr
type textPage uintptr

var (
	lib uintptr

	FPDF_InitLibrary    func()
	FPDF_DestroyLibrary func()

	FPDF_LoadMemDocument func(data []byte, length uint64, password *byte) document
	FPDF_LoadDocument    func(path string, password uintptr) document
	// document
	FPDF_GetPageCount  func(docHandle document) int
	FPDF_CloseDocument func(docHandle document)
	//Get the file version of the specific PDF document.
	FPDF_GetFileVersion func(docHandle document, result *int) bool

	// page
	FPDF_LoadPage         func(docHandle document, index int32) page
	FPDF_ClosePage        func(pageHandle page)
	FPDFPage_CountObjects func(pageHandle page) int32
	FPDFPage_GetObject    func(pageHandle page, index int32) (pageObjectHandle uintptr)
	FPDFPageObj_GetType   func(objHandle uintptr) int32

	FPDFImageObj_GetImageDataDecoded func(objHandle uintptr, resultBuffer []byte, bufLength uint64) (length uint64)
	// text
	FPDFText_LoadPage   func(page) textPage
	FPDFText_ClosePage  func(textPage)
	FPDFText_CountChars func(textPage) int

	// Extract unicode text string from the page, in UTF-16LE encoding.
	// Returns Number of characters written into parameter result buffer, excluding the trailing terminator.
	FPDFText_GetText    func(textHandle textPage, startIndex int, count int, resultBuf []byte) (charsWritten int)
	FPDFText_GetUnicode func(textHandle textPage, index int) rune
	/*
			Get the text string of specific tag from meta data of a PDF document.

		Regardless of the platform, the text string is alway in UTF-16LE encoding.
		That means the buffer can be treated as an array of WORD (on Intel and compatible CPUs),
		each WORD representing the Unicode of a character(some special Unicode may take 2 WORDs).
		The string is followed by two bytes of zero indicating the end of the string.
	*/
	FPDF_GetMetaText func(documenthandle document, tag string, resultBuf []byte, bufLength uint64) (bytesNeeded uint64)

	// PDFium is not thread-safe. This lock guards the lib against concurrent access in places where this is known to be necessary
	Lock sync.Mutex
)

type Document struct {
	handle document
	path   string
	data   *[]byte
	pages  int
}

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
	purego.RegisterLibFunc(&FPDF_InitLibrary, lib, "FPDF_InitLibrary")

	purego.RegisterLibFunc(&FPDF_DestroyLibrary, lib, "FPDF_DestroyLibrary")
	purego.RegisterLibFunc(&FPDF_LoadMemDocument, lib, "FPDF_LoadMemDocument")
	purego.RegisterLibFunc(&FPDF_LoadDocument, lib, "FPDF_LoadDocument")

	purego.RegisterLibFunc(&FPDF_CloseDocument, lib, "FPDF_CloseDocument")
	purego.RegisterLibFunc(&FPDF_GetFileVersion, lib, "FPDF_GetFileVersion")
	purego.RegisterLibFunc(&FPDF_GetPageCount, lib, "FPDF_GetPageCount")
	purego.RegisterLibFunc(&FPDF_LoadPage, lib, "FPDF_LoadPage")
	purego.RegisterLibFunc(&FPDF_ClosePage, lib, "FPDF_ClosePage")

	purego.RegisterLibFunc(&FPDFPage_CountObjects, lib, "FPDFPage_CountObjects")
	purego.RegisterLibFunc(&FPDFPage_GetObject, lib, "FPDFPage_GetObject")
	purego.RegisterLibFunc(&FPDFPageObj_GetType, lib, "FPDFPageObj_GetType")
	purego.RegisterLibFunc(&FPDFImageObj_GetImageDataDecoded, lib, "FPDFImageObj_GetImageDataDecoded")

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
	handle := FPDF_LoadMemDocument(data, uint64(len(data)), nil)
	if handle == 0 {
		return nil, errors.New("pdfium: cannot load document")
	}
	return &Document{data: &data, handle: handle, pages: FPDF_GetPageCount(handle)}, nil
}

func Open(path string) (*Document, error) {
	handle := FPDF_LoadDocument(path, 0)
	if handle == 0 {
		return nil, errors.New("pdfium: cannot load document")
	}
	return &Document{data: nil, path: path, handle: handle, pages: FPDF_GetPageCount(handle)}, nil
}

func (d *Document) Close() {
	if d.handle != 0 {
		FPDF_CloseDocument(d.handle)
		d.handle = 0
	}
	d.data = nil
}

func (d *Document) Pages() int {
	return d.pages
}

func (d *Document) Data() *[]byte {
	return d.data
}

func (d *Document) Path() string {
	return d.path
}

func (d *Document) page(i int) page {
	Lock.Lock()
	defer Lock.Unlock()
	p := FPDF_LoadPage(d.handle, int32(i))
	return p
}

func (p page) close() {
	Lock.Lock()
	defer Lock.Unlock()
	FPDF_ClosePage(p)
}

func (d *Document) textPage(pageHandle page) textPage {
	Lock.Lock()
	t := FPDFText_LoadPage(pageHandle)
	defer Lock.Unlock()
	return t
}

func (t textPage) close() {
	Lock.Lock()
	defer Lock.Unlock()
	FPDFText_ClosePage(t)
}

func (t textPage) countChars() int {
	Lock.Lock()
	defer Lock.Unlock()
	chars := FPDFText_CountChars(t)
	return chars
}

func (t textPage) utf16Text() []byte {
	charCount := t.countChars()
	charData := make([]byte, (charCount+1)*2)
	Lock.Lock()
	defer Lock.Unlock()
	charsWritten := FPDFText_GetText(t, 0, charCount, charData)
	// strip 2 trailing NUL bytes
	return charData[0 : 2*charsWritten]
}

func (t textPage) utf8Text() string {
	charData := t.utf16Text()
	if len(charData) == 0 {
		return ""
	}
	result, _ := transformUtf16LeToUtf8(charData)
	// PDFium inserts NUL bytes around headers and footers
	result = strings.ReplaceAll(result, "\x00", "\n")
	// PDFium replaces hyphens with `noncharacters`:
	result = strings.ReplaceAll(result, "\uFFFE", "")
	result = strings.TrimSpace(result)
	return result
}

// Text returns page i's text
func (d *Document) Text(i int) (string, bool) {
	p := d.page(i)
	defer p.close()
	t := d.textPage(p)
	defer t.close()
	text := t.utf8Text()
	hasImages := false
	if len(text) == 0 && d.countImages(p) > 0 {
		hasImages = true
	}
	return text, hasImages
}

func (d *Document) StreamText(w io.Writer) error {
	for i := range d.pages {
		pageText, _ := d.Text(i)
		_, err := w.Write([]byte(pageText))
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *Document) MetadataMap() map[string]string {
	m := make(map[string]string)
	m["x-parsed-by"] = "PDFium"
	m["x-doctype"] = "pdf"
	m["x-document-pages"] = strconv.Itoa(d.pages)

	var version int
	if ok := FPDF_GetFileVersion(d.handle, &version); ok {
		// the result is 18 for version 1.8 etc
		m["x-document-version"] = fmt.Sprintf("PDF-%.1f", float32(version)/10)
	}

	// we use the same (oversized) byte array for all fields
	var defaultSize uint64 = 512
	buf := make([]byte, defaultSize)
	lookup := func(key string) (ok bool, value string) {
		requiredSize := FPDF_GetMetaText(d.handle, key, buf, uint64(len(buf)))
		if requiredSize <= 2 {
			return false, ""
		}
		if requiredSize > uint64(len(buf)) {
			// if the buffer was too small, allocate a bigger one
			buf = make([]byte, requiredSize)
			FPDF_GetMetaText(d.handle, key, buf, uint64(len(buf)))
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

func (d *Document) countImages(pageHandle page) int {
	Lock.Lock()
	defer Lock.Unlock()
	objCount := FPDFPage_CountObjects(pageHandle)
	imgCount := 0
	for i := range objCount {
		if obj := FPDFPage_GetObject(pageHandle, i); FPDFPageObj_GetType(obj) == 3 {
			imgCount++
		}
	}
	return imgCount
}
