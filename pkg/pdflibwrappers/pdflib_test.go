package pdflibwrappers_test

import (
	"os"
	"testing"

	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/mupdf_purego"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/pdfium_purego"
	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/poppler_purego"
)

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
	defer d.Close()
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

func TestPoppler(t *testing.T) {
	_, err := poppler_purego.InitLib("")
	if err != nil {
		t.Skipf("poppler could not be loaded: %v", err)
	}
	d, err := poppler_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
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

func TestMuPdf(t *testing.T) {
	_, err := mupdf_purego.InitLib("")
	if err != nil {
		t.Skipf("mupdf could not be loaded: %v", err)
	}
	d, err := mupdf_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	txt, _ := d.Text(0)
	// determining images on page is not implemented, so second return value is always true
	checkTextLength(t, txt)
	t.Log(txt)
	meta := d.MetadataMap()
	checkMetadataEntries(t, meta)
	t.Log(meta)
	if d.Pages() != 2 {
		t.Errorf("expected to find 2 pages but were %d", d.Pages())
	}
}

func readFile() []byte {
	data, err := os.ReadFile("testdata/2000001.pdf")
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
