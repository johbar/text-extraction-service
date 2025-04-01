package officexmlparser

import (
	"archive/zip"
	"io"
	"os"
	"strings"
	"testing"
)

const (
	beginning     = "text-extraction-service\nTES is a simple Go service for extracting and storing textual content from PDF, RTF and legacy MS Word (.doc) documents."
	lastLine      = "Nevertheless, if you intend to fork this project and remove the GPL-related code in favor of PDFium only, feel free to put it under Apache-2 license.\n"
	lastLineSlide = "(Experimental) Optical character recognition by Tesseract OCR (useful for images containing text and scanned PDFs)\n"
)

var metadata = map[string]string{
	// "x-document-author": "johbar",
	// "x-document-created":"2025-02-01T22:59:45.190928424",
	"x-document-keywords": "PDF word document text extraction",
	// "x-document-modified": "2025-02-06T19:21:38.110782103",
	// "x-document-pages":    "2",
	// "x-document-paragraphs": "31",
	"x-document-producer": "LibreOffice/24.2.7.2$Linux_X86_64 LibreOffice_project/420$Build-2",
	"x-document-subject":  "Text extraction service",
	"x-document-title":    "README of github.com/johbar/text-extraction-service",
}

func TestOdt(t *testing.T) {
	testReadmeBytes(t, "odt")
	testReadmeFile(t, "odt")
}

func TestOdp(t *testing.T) {
	testReadmeBytes(t, "odp")
	testReadmeFile(t, "odp")
}

func TestDocx(t *testing.T) {
	testReadmeBytes(t, "docx")
	testReadmeFile(t, "docx")
}

func TestPptx(t *testing.T) {
	testReadmeBytes(t, "pptx")
	testReadmeFile(t, "pptx")
}

func TestOpenTextMetadata(t *testing.T) {
	m := make(map[string]string)
	data, err := extractNamedFile("testdata/readme.odt", "meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range m {
		if len(v) == 0 && k != "x-document-creator" {
			t.Errorf("Expected %s to be non-empty", k)
		}
	}
	mapOpenDocumentMetadata(m, data)
	t.Log(m)

}

func testReadmeFile(t *testing.T, ext string) {
	d, err := Open("testdata/readme."+ext, ext)
	if err != nil {
		t.Fatal(err)
	}
	testReadme(t, d)
	if len(d.Path()) < 1 {
		t.Errorf("expected path to be non-empty")
	}
}

func testReadmeBytes(t *testing.T, ext string) {
	f, err := os.ReadFile("testdata/readme." + ext)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewFromBytes(f, ext)
	if err != nil {
		t.Fatal(err)
	}
	testReadme(t, d)
	if len(d.Path()) > 0 {
		t.Errorf("expected path to be empty")
	}
}

func testReadme(t *testing.T, d *XmlBasedDocument) {
	var sb strings.Builder
	err := d.StreamText(&sb)
	if err != nil {
		t.Fatal(err)
	}
	result := sb.String()
	t.Log(result)
	t.Logf("Metadata: %v", d.MetadataMap())
	if !strings.HasPrefix(result, beginning) {
		t.Errorf("Extracted text did not start as expected")
	}
	if d.ext == "pptx" || d.ext == "odp" {
		if !strings.HasSuffix(result, lastLineSlide) {
			t.Errorf("Extracted text did not end as expected")
		}
	} else {
		if !strings.HasSuffix(result, lastLine) {
			t.Errorf("Extracted text did not end as expected")
		}
	}
	checkMetaData(d.MetadataMap(), t)
	d.Close()
}

func extractNamedFile(path string, pathInZip string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	r, err := zr.Reader.Open(pathInZip)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func checkMetaData(m map[string]string, t *testing.T) {
	for k, v := range metadata {
		if m[k] != v {
			t.Errorf("field '%s': expected: '%s', got: '%s'", k, v, m[k])
		}
	}
}
