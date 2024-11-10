package puregopoppler

type DocumentInfo struct {
	PdfVersion, Title, Author, Subject, KeyWords, Creator, Producer, Metadata string
	CreationDate, ModificationDate                                            int64
	Pages                                                                     int
}

type GError struct {
	domain  uint32
	code    int32
	message *byte
}

// Page represents a PDF page opened by Poppler
type Page struct {
	uintptr
}

// Document represents a PDF opened by Poppler
type Document struct {
	uintptr
}
