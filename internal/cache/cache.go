package cache

import (
	"io"

	"github.com/nats-io/nats.go/jetstream"
)

// Document represents any kind of document this service can convert to plain text
type Document interface {
	// StreamText writes all text to w
	StreamText(w io.Writer) error
	// Pages returns the documents number of pages. Returns -1 if the concept is not applicable to the file type.
	Pages() int
	// Text returns a single page's text and true if there is at least one image on the page
	Text(int) (string, bool)
	// Data returns the underlying byte array or nil if the document was loaded from disk
	Data() *[]byte
	// Path returns the filesystem path a document was loaded from or an empty string if the was not loaded from disk
	Path() string
	// MetadataMap returns a map of Document properties, such as Author, Title etc.
	MetadataMap() DocumentMetadata
	// Close releases resources associated with the document
	Close()
}

type DocumentMetadata = map[string]string

// ExtractedDocument contains pointers to metadata, textual content and URL of origin
type ExtractedDocument struct {
	Url      *string
	Metadata *map[string]string
	Text     []byte
	Doc      Document
}

type Cache interface {
	GetMetadata(url string) (DocumentMetadata, error)
	StreamText(url string, w io.Writer) error
	Save(doc ExtractedDocument) (*jetstream.ObjectInfo, error)
}

type NopCache struct{}

func (c *NopCache) GetMetadata(url string) (DocumentMetadata, error) {
	return nil, nil
}

func (c *NopCache) StreamText(url string, w io.Writer) error {
	return nil
}

func (c *NopCache) Save(doc ExtractedDocument) (*jetstream.ObjectInfo, error) {
	return &jetstream.ObjectInfo{}, nil
}
