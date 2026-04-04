package rtfparser

import (
	"io"
	"strings"
	"testing"
)

const (
	beginning = "text-extraction-service\nTES is a simple Go service for extracting and storing textual content from PDF, RTF and legacy MS Word (.doc) documents."
	lastLine  = "Nevertheless, if you intend to fork this project and remove the GPL-related code in favor of PDFium only, feel free to put it under Apache-2 license.\n"
)

func TestRtf(t *testing.T) {
	filePath := "testdata/readme.rtf"
	d, err := Open(filePath)
	if err != nil {
		t.Fatalf("opening RTF file failed: %v", err)
	}
	metadata, err := ExtractMetadata(d.input)
	d.input.Seek(0, io.SeekStart)
	t.Log(metadata)
	if err != nil {
		t.Fatalf("metadata extraction failed: %v", err)
	}
	if metadata.Title != "README of github.com/johbar/text-extraction-service" {
		t.Errorf("Expected title to be 'README of github.com/johbar/text-extraction-service', but was %s", metadata.Title)
	}
	txt, _ := d.Text(0)
	t.Log(txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content did not end as expected")
	}
}
