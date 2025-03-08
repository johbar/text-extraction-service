package docparser

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const (
	beginning = "text-extraction-service TES is a simple Go service for extracting and storing textual content from PDF, RTF and legacy MS Word (.doc) documents."
	lastLine  = "Nevertheless, if you intend to fork this project and remove the GPL-related code in favor of PDFium only, feel free to put it under Apache-2 license. "
	filePath  = "testdata/readme.doc"
)

func TestDocParser(t *testing.T) {
	if !Initialized {
		t.Skip("no doc parser tool found")
	}
	data, _ := os.ReadFile(filePath)
	d, err := NewFromBytes(data)
	if err != nil {
		t.Fail()
	}
	t.Log(d.metadata)
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
		t.Errorf("Extracted content did not end as expected")
	}
}

func TestAntiword(t *testing.T) {
	if _, err := exec.LookPath(antiword.cmd); err != nil {
		return
	}
	data, _ := os.ReadFile(filePath)
	d, err := NewFromBytes(data)
	if err != nil {
		t.Fail()
	}

	var buf bytes.Buffer
	if err := d.runExternalWordProcessor(&buf, antiword); err != nil {
		t.Fatal(err)
	}
	txt := buf.String()
	t.Log(txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content did not end as expected")
	}
}

func TestCatdoc(t *testing.T) {
	if _, err := exec.LookPath(catdoc.cmd); err != nil {
		return
	}
	data, _ := os.ReadFile(filePath)
	d, err := NewFromBytes(data)
	if err != nil {
		t.Fail()
	}

	var buf bytes.Buffer
	if err := d.runExternalWordProcessor(&buf, catdoc); err != nil {
		t.Fatal(err)
	}
	txt := buf.String()
	t.Log(txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content did not end as expected")
	}
}

func TestWvWare(t *testing.T) {
	if _, err := exec.LookPath(wvWare.cmd); err != nil {
		return
	}
	data, _ := os.ReadFile(filePath)
	d, err := NewFromBytes(data)
	if err != nil {
		t.Fail()
	}

	var buf bytes.Buffer
	if err := d.runExternalWordProcessor(&buf, wvWare); err != nil {
		t.Fatal(err)
	}
	txt := buf.String()
	t.Log(txt)
	if !strings.HasPrefix(txt, beginning) {
		t.Errorf("Extracted content did not start as expected")
	}
	if !strings.HasSuffix(txt, lastLine) {
		t.Errorf("Extracted content did not end as expected")
	}
}