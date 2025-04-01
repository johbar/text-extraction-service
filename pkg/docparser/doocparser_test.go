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
var readmeBytes []byte

func init() {
	var err error
	readmeBytes, err = os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
}

func TestDocParser(t *testing.T) {
	if !Initialized {
		t.Skip("no doc parser tool found")
	}
	d, err := NewFromBytes(readmeBytes)
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

func TestMetadata(t *testing.T) {
	d, err := NewFromBytes(readmeBytes)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("metadata: %+v", d.metadata)
	if d.metadata.Title != "README of github.com/johbar/text-extraction-service" {
		t.Errorf("Expected author to be 'README of github.com/johbar/text-extraction-service', but was %s", d.metadata.Title)
	}}

func TestAntiword(t *testing.T) {
	if _, err := exec.LookPath(antiword.cmd); err != nil {
		t.Skip("antiword not found")
	}
	d, err := NewFromBytes(readmeBytes)
	if err != nil {
		t.Fail()
	}

	testExternalWordProcessor(t, d, antiword)
	d, err = Open(filePath)
	if err != nil {
		t.Fail()
	}
testExternalWordProcessor(t, d, antiword)}

func TestCatdoc(t *testing.T) {
	if _, err := exec.LookPath(catdoc.cmd); err != nil {
		t.Skip("catdoc not found")
	}
	d, err := NewFromBytes(readmeBytes)
	if err != nil {
		t.Fail()
	}
	testExternalWordProcessor(t, d, catdoc)
	d, err = Open(filePath)
	if err != nil {
		t.Fail()
	}
	testExternalWordProcessor(t, d, catdoc)
}

func TestWvWare(t *testing.T) {
	if _, err := exec.LookPath(wvWare.cmd); err != nil {
		t.Skip("wvWare not found")
	}
	d, err := NewFromBytes(readmeBytes)
	if err != nil {
		t.Fail()
	}
	testExternalWordProcessor(t, d, wvWare)
	d, err = Open(filePath)
	if err != nil {
		t.Fail()
	}
	testExternalWordProcessor(t, d, wvWare)
}

func testExternalWordProcessor(t *testing.T, d *WordDoc, p progAndArgs) {
	var buf bytes.Buffer
	if err := d.runExternalWordProcessor(&buf, p); err != nil {
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
	d.Close()
}