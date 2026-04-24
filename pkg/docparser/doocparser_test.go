package docparser

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

const (
	beginning = "text-extraction-service\nTES is a simple Go service for extracting and storing textual content from PDF, RTF and legacy MS Word (.doc) documents."
	lastLine  = "Nevertheless, if you intend to fork this project and remove the GPL-related code in favor of PDFium only, feel free to put it under Apache-2 license.\n"
	filePath  = "testdata/readme.doc"
)

var readmeBytes []byte

func init() {
	var err error
	readmeBytes, err = os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
}

func TestDocParser(t *testing.T) {
	d, err := NewFromBytes(readmeBytes)
	if err != nil {
		t.Fail()
	}
	metadata, err := d.docStreams.metadata()
	if err != nil {
		t.Errorf("Metadata extraction failed: %v", err)
	}
	t.Log(metadata)
	if metadata.Title != "README of github.com/johbar/text-extraction-service" {
		t.Errorf("Expected author to be 'README of github.com/johbar/text-extraction-service', but was %s", metadata.Title)
	}
	var buf bytes.Buffer
	if err := d.StreamText(&buf); err != nil {
		t.Fatal(err)
	}
	txt := buf.String()
	t.Logf("'%s'", txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content did not end as expected")
	}
}

func TestPptParser(t *testing.T) {
	d, err := Open("testdata/readme.ppt")
	if err != nil {
		t.Fatal(err)
	}
	if d.docStreams == nil {
		t.Fatal("pptRaw is nil")
	}
	if d.docStreams.pptDoc == nil {
		t.Fatal("pptDoc is nil")
	}
	if d.docStreams.currentUser == nil {
		t.Fatal("currentUser is nil")
	}
	metadata, err := d.docStreams.metadata()
	if err != nil {
		t.Errorf("Metadata extraction failed: %v", err)
	}
	t.Log(metadata)
	if metadata.Title != "README of github.com/johbar/text-extraction-service" {
		t.Errorf("Expected author to be 'README of github.com/johbar/text-extraction-service', but was %s", metadata.Title)
	}
	var buf bytes.Buffer
	if err := d.StreamText(&buf); err != nil {
		t.Fatal(err)
	}
	txt := buf.String()
	t.Logf("'%s'", txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content did not end as expected")
	}
}
