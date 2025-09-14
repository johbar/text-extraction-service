package docfactory

import (
	"os"
	"reflect"
	"testing"

	"github.com/johbar/text-extraction-service/v4/internal/config"
	"github.com/johbar/text-extraction-service/v4/pkg/docparser"
	"github.com/johbar/text-extraction-service/v4/pkg/officexmlparser"
	"github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers/pdfium_purego"
	"github.com/johbar/text-extraction-service/v4/pkg/rtfparser"
)

const readmeOcrPath = "../../pkg/pdflibwrappers/testdata/readme.pdf"

func TestNewFromPath(t *testing.T) {
	var (
		xmltyp    *officexmlparser.XmlBasedDocument
		doctyp    *docparser.WordDoc
		rtftyp    *rtfparser.RichTextDoc
		pdfiumtyp *pdfium_purego.Document
	)
	conf, err := config.NewTesConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	df := New(conf, nil)
	var cases = []struct {
		path string
		typ  reflect.Type
	}{
		{"../../pkg/officexmlparser/testdata/readme.docx", reflect.TypeOf(xmltyp)},
		{"../../pkg/officexmlparser/testdata/readme.odt", reflect.TypeOf(xmltyp)},
		{"../../pkg/officexmlparser/testdata/readme.pptx", reflect.TypeOf(xmltyp)},
		{"../../pkg/officexmlparser/testdata/readme.odp", reflect.TypeOf(xmltyp)},
		{"../../pkg/docparser/testdata/readme.doc", reflect.TypeOf(doctyp)},
		{"../../pkg/rtfparser/testdata/readme.rtf", reflect.TypeOf(rtftyp)},
		{readmeOcrPath, reflect.TypeOf(pdfiumtyp)},
	}
	for _, doc := range cases {

		d, err := df.NewFromPath(doc.path, doc.path)
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
	conf, err := config.NewTesConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	// a large threshold ensures the file is being loaded in memory
	conf.MaxInMemoryBytes = 10_000_000
	df := New(conf, nil)
	path := readmeOcrPath
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := df.handleUnknownSize(f, path)
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
	conf, err := config.NewTesConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	// a small threshold ensures the file is being loaded on disk
	conf.MaxInMemoryBytes = 100
	df := New(conf, nil)
	path := readmeOcrPath
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := df.handleUnknownSize(f, path)
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
