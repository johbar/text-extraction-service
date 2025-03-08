package rtfparser

import (
	"io"
	"os"
	"strings"
	"testing"
)

const (
	beginning = "text-extraction-service\nTES is a simple Go service for extracting and storing textual content from PDF, RTF and legacy MS Word (.doc) documents."
	lastLine  = "Nevertheless, if you intend to fork this project and remove the GPL-related code in favor of PDFium only, feel free to put it under Apache-2 license.\n"
)

func TestRtf(t *testing.T) {
	filePath := "testdata/readme.rtf"
	f, _ := os.Open(filePath)
	_, _ =f.Seek(0, 0)
	data, _ := io.ReadAll(f)
	d, err := NewFromBytes(data)
	t.Log(d.metadata)
	if err != nil {
		t.Fail()
	}
	if d.metadata.Title != "README of github.com/johbar/text-extraction-service" {
		t.Errorf("Expected author to be 'README of github.com/johbar/text-extraction-service', but was %s", d.metadata.Title)
	}
	txt, _ := d.Text(0)
	t.Log(txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content dit not end as expected")
	}
}
