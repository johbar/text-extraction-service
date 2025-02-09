package docparser

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

const (
	beginning = "text-extraction-service TES is a simple Go service for extracting and storing textual content from PDF, RTF and legacy MS Word (.doc) documents."
	lastLine  = "Nevertheless, if you intend to fork this project and remove the GPL-related code in favor of PDFium only, feel free to put it under Apache-2 license. "
)

func TestDoc(t *testing.T) {
	if Initialized == false {
		return
	}
	filePath := "testdata/readme.doc"
	f, _ := os.Open(filePath)
	data, _ := io.ReadAll(f)
	d, err := NewFromBytes(data)
	t.Log(d.metadata)
	if err != nil {
		t.Fail()
	}
	if d.metadata.Title != "README of github.com/johbar/text-extraction-service" {
		t.Errorf("Expected author to be 'README of github.com/johbar/text-extraction-service', but was %s", d.metadata.Title)
	}
	var buf bytes.Buffer
	if err := d.StreamText(&buf); err != nil {
		t.Fatal(err)
	}
	txt := buf.String()
	t.Log(txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content dit not end as expected")
	}
}
