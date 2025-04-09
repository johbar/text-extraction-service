package main

import (
	"log/slog"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/johbar/text-extraction-service/v2/pkg/docparser"
	"github.com/johbar/text-extraction-service/v2/pkg/officexmlparser"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/pdfium_purego"
	"github.com/johbar/text-extraction-service/v2/pkg/rtfparser"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

const readmeOcrPath = "pkg/pdflibwrappers/testdata/readme.pdf"

func TestWriteTextOrRunOcr(t *testing.T) {
	tesConfig = NewTesConfigFromEnv()
	// we need to create a logger, otherwise it's a nil pointer
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}))
	err := LoadPdfLib("pdfium", "")
	if err != nil {
		t.Skip(err)
	}
	pdfImpl = pdfImplementation{libShort: "pdfium"}
	tesswrap.Languages = "eng"

	f, err := os.Open(readmeOcrPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	doc, err := NewDocFromStream(f, -1, readmeOcrPath)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	err = WriteTextOrRunOcr(doc, &sb, readmeOcrPath)
	if err != nil {
		t.Error(err)
	}
	t.Log(sb.String())
	if sb.Len() < 100 {
		t.Error("expected more text as a result of OCR")
	}
	if len(doc.Path()) > 0 {
		// temp file
		os.Remove(doc.Path())
	}
}

func TestNewFromPath(t *testing.T) {
	var xmltyp *officexmlparser.XmlBasedDocument
	var doctyp *docparser.WordDoc
	var rtftyp *rtfparser.RichTextDoc
	var pdfiumtyp *pdfium_purego.Document

	_ = LoadPdfLib("pdfium", "")
	var cases = []struct {
		path string
		typ  reflect.Type
	}{
		{"pkg/officexmlparser/testdata/readme.docx", reflect.TypeOf(xmltyp)},
		{"pkg/officexmlparser/testdata/readme.odt", reflect.TypeOf(xmltyp)},
		{"pkg/officexmlparser/testdata/readme.pptx", reflect.TypeOf(xmltyp)},
		{"pkg/officexmlparser/testdata/readme.odp", reflect.TypeOf(xmltyp)},
		{"pkg/docparser/testdata/readme.doc", reflect.TypeOf(doctyp)},
		{"pkg/rtfparser/testdata/readme.rtf", reflect.TypeOf(rtftyp)},
		{readmeOcrPath, reflect.TypeOf(pdfiumtyp)},
	}
	for _, doc := range cases {

		d, err := NewFromPath(doc.path, doc.path)
		if err != nil {
			t.Error(err)
		}
		if reflect.TypeOf(d) != doc.typ {
			t.Errorf("expected document to be of type %v, but was %v", doc.typ, reflect.TypeOf(d))
		}
		if d.Path() != doc.path {
			t.Errorf("expected path to be '%s', but was '%s'", doc.path, d.Path())
		}
		if d.Data() != nil {
			t.Error("expected Data() to return nil")
		}
		d.Close()
	}
}

func TestUnknownSizedStreamEmitsData(t *testing.T) {
	tesConfig = NewTesConfigFromEnv()
	tesConfig.maxInMemoryBytes = 10_000_000
	path := readmeOcrPath
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := handleUnknownSize(f, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Path()) > 0 {
		t.Fatalf("want no path, got %v", d.Path())
	}
	stat, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if len(*d.Data()) != int(stat.Size()) {
		t.Errorf("want %d bytes, got %d", len(*d.Data()), stat.Size())
	}
}

func TestUnknownSizedStreamEmitsFile(t *testing.T) {
	tesConfig = NewTesConfigFromEnv()
	tesConfig.maxInMemoryBytes = 100
	path := readmeOcrPath
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := handleUnknownSize(f, path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if len(d.Path()) < 1 {
		t.Fatalf("want path, got %v", d.Path())
	}
	defer os.Remove(d.Path())
	if d.Data() != nil {
		t.Fatal("want nil, got data")
	}
	stat, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	stat2, err := os.Stat(d.Path())
	if err != nil {
		t.Fatal(err)
	}
	if stat2.Size() != stat.Size() {
		t.Errorf("temp file not the same size as original file %d != %d", stat2.Size(), stat.Size())
	}
}
