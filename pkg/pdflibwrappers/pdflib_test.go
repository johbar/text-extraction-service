package pdflibwrappers_test

import (
	"io"
	"os"
	"testing"

	mupdfpure "github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/mupdf_purego"
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
		t.Fatalf("pdfium could not be loaded: %v", err)
	}
	d, err := pdfium_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	txt, _ := d.Text(0)
	checkTextLength(t, txt)
	t.Log(txt)
	meta := d.MetadataMap()
	checkMetadataEntries(t, meta)
	t.Log(meta)
}

func TestPoppler(t *testing.T) {
	_, err := poppler_purego.InitLib("")
	if err != nil {
		t.Fatalf("poppler could not be loaded: %v", err)
	}
	d, err := poppler_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	txt, _:= d.Text(0)
	checkTextLength(t, txt)
	t.Log(txt)
	meta := d.MetadataMap()
	checkMetadataEntries(t, meta)
	t.Log(meta)
}

func TestMuPdf(t *testing.T) {
	_, err := mupdfpure.InitLib("")
	if err != nil {
		t.Fatalf("poppler could not be loaded: %v", err)
	}
	d, err := poppler_purego.Load(pdfFile)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	txt, _:= d.Text(0)
	checkTextLength(t, txt)
	t.Log(txt)
	meta := d.MetadataMap()
	checkMetadataEntries(t, meta)
	t.Log(meta)
}

func readFile() []byte {
	f, err := os.Open("testdata/2000001.pdf")
	if err != nil {
		panic(err)
	}
	data, _ := io.ReadAll(f)
	f.Close()
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
		t.Errorf("Not enough entries in metadata map: %d. Expected: 10", metaLen)
	}
}
