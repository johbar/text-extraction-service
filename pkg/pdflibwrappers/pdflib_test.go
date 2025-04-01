package pdflibwrappers_test

import (
	"os"
	"testing"

	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/mupdf_purego"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/pdfium_purego"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/poppler_purego"
)

const shortPdf = "testdata/2000001.pdf"

var (
	pdfFile []byte
)

func init() {
	pdfFile = readFile()
}

func TestPdfium(t *testing.T) {
	_, err := pdfium_purego.InitLib("")
	if err != nil {
		t.Skipf("pdfium could not be loaded: %v", err)
	}
	d, err := pdfium_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	if d.Path() != "" {
		t.Errorf("expected PDF path to be empty, but was '%s'", d.Path())
	}
	runt := func() {
		txt, hasImages := d.Text(0)
		if hasImages {
			t.Errorf("expected not to find images on page")
		}
		checkTextLength(t, txt)
		t.Log(txt)
		meta := d.MetadataMap()
		checkMetadataEntries(t, meta)
		t.Log(meta)
		if d.Pages() != 2 {
			t.Errorf("expected to find 2 pages but were %d", d.Pages())
		}
	}
	runt()
	d.Close()
	d, err = pdfium_purego.Open(shortPdf)
	if err != nil {
		t.Fatal("pdfium could not load file from path", err)
	}
	defer d.Close()
	if d.Path() != shortPdf {
		t.Errorf("expected PDF path to be '%s', but was '%s'", shortPdf, d.Path())
	}
	runt()
}

func TestPoppler(t *testing.T) {
	_, err := poppler_purego.InitLib("")
	if err != nil {
		t.Skipf("poppler could not be loaded: %v", err)
	}
	d, err := poppler_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	if d.Path() != "" {
		t.Errorf("expected PDF path to be empty, but was '%s'", d.Path())
	}
	runt := func() {
		txt, hasImages := d.Text(0)
		if hasImages {
			t.Errorf("expected not to find images on page")
		}
		checkTextLength(t, txt)
		t.Log(txt)
		meta := d.MetadataMap()
		checkMetadataEntries(t, meta)
		t.Log(meta)
		if d.Pages() != 2 {
			t.Errorf("expected to find 2 pages but were %d", d.Pages())
		}
	}
	runt()
	d.Close()
	d, err = poppler_purego.Open(shortPdf)
	if err != nil {
		t.Fatal("poppler could not load file from path", err)
	}
	if d.Path() != shortPdf {
		t.Errorf("expected PDF path to be '%s', but was '%s'", shortPdf, d.Path())
	}
	runt()
	d.Close()
}

func TestMuPdf(t *testing.T) {
	_, err := mupdf_purego.InitLib("")
	if err != nil {
		t.Skipf("mupdf could not be loaded: %v", err)
	}
	d, err := mupdf_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	if d.Path() != "" {
		t.Errorf("expected PDF path to be empty, but was '%s'", d.Path())
	}
	runt := func() {
		txt, _ := d.Text(0)
		checkTextLength(t, txt)
		t.Log(txt)
		meta := d.MetadataMap()
		checkMetadataEntries(t, meta)
		t.Log(meta)
		if d.Pages() != 2 {
			t.Errorf("expected to find 2 pages but were %d", d.Pages())
		}
	}
	runt()
	d.Close()
	d, err = mupdf_purego.Open(shortPdf)
	if err != nil {
		t.Fatal("mupdf could not load file from path", err)
	}
	if d.Path() != shortPdf {
		t.Errorf("expected PDF path to be '%s', but was '%s'", shortPdf, d.Path())
	}
	runt()
	d.Close()
}

func readFile() []byte {
	data, err := os.ReadFile(shortPdf)
	if err != nil {
		panic(err)
	}
	return data
}

func checkTextLength(t *testing.T, txt string) {
	if len(txt) < 30 {
		t.Errorf("Text of page 0 too short: %d", len(txt))
	}
}

func checkMetadataEntries(t *testing.T, meta map[string]string) {
	metaLen := len(meta)
	if metaLen != 10 {
		t.Errorf("expected to find 10 entries in metadata map, but were %d", metaLen)
	}
	if meta["x-document-title"] != "Drucksache 20/1" {
		t.Errorf("expected document title to be 'Drucksache 20/1', but was '%s'", meta["x-document-title"])
	}
}
