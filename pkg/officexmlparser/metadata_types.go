package officexmlparser

// DublinCoreMetadata represents metadata that is identical in
// Open Document and MS Office files
type DublinCoreMetadata struct {
	Creator     string `xml:"creator"`
	Date        string `xml:"date"`
	Description string `xml:"description"`
	Language    string `xml:"language"`
	Publisher   string `xml:"publisher"`
	Rights      string `xml:"rights"`
	Subject     string `xml:"subject"`
	Title       string `xml:"title"`
}

// OpenDocumentMetadata represents metadata found in Open Document files
type OpenDocumentMetadata struct {
	Meta struct {
		DublinCoreMetadata
		CreationDate string            `xml:"creation-date"`
		Generator    string            `xml:"generator"`
		Keywords     []string          `xml:"keyword"`
		Stats        OpenDocumentStats `xml:"document-statistic"`
	} `xml:"meta"`
}

// OpenDocumentStats represents statistical metadata stored in an Open Document file
type OpenDocumentStats struct {
	CharCount      string `xml:"character-count,attr"`
	ImageCount     string `xml:"image-count,attr"`
	ObjectCount    string `xml:"object-count,attr"`
	PageCount      string `xml:"page-count,attr"`
	ParagraphCount string `xml:"paragraph-count,attr"`
	WordCount      string `xml:"word-count,attr"`
}
// MsOfficeCoreMetadata represents common metadata found in MS Office files
type MsOfficeCoreMetadata struct {
	DublinCoreMetadata
	Keywords string `xml:"keywords"`
	Modified string `xml:"modified"`
	Created  string `xml:"created"`
}

type MsOfficeStats struct {
	Generator      string `xml:"Application"`
	PageCount      string `xml:"Pages"`
	WordCount      string `xml:"Words"`
	ParagraphCount string `xml:"Paragraphs"`
	CharCount      string `xml:"CharactersWithSpaces"`
}
