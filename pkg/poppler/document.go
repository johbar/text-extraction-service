package puregopoppler

import "runtime"

func (d *Document) GetNPages() int {
	return poppler_document_get_n_pages(d.uintptr)
}

// GetPage opens a PDF page by index (zero-based)
func (d *Document) GetPage(i int) *Page {
	p:= &Page{poppler_document_get_page(d.uintptr, i)}
	runtime.SetFinalizer(p, closePage)
	return p
}

func (d *Document) Close() {
	g_object_unref(d.uintptr)
	d.uintptr = 0
}

// Info returns document metadata
func (d *Document) Info() DocumentInfo {
	return DocumentInfo{
		PdfVersion:       toStr(poppler_document_get_pdf_version_string(d.uintptr)),
		Title:            toStr(poppler_document_get_title(d.uintptr)),
		Author:           toStr(poppler_document_get_author(d.uintptr)),
		Subject:          toStr(poppler_document_get_subject(d.uintptr)),
		KeyWords:         toStr(poppler_document_get_keywords(d.uintptr)),
		Creator:          toStr(poppler_document_get_creator(d.uintptr)),
		CreationDate:     poppler_document_get_creation_date(d.uintptr),
		ModificationDate: poppler_document_get_modification_date(d.uintptr),
		Pages:            poppler_document_get_n_pages(d.uintptr),
	}
}
