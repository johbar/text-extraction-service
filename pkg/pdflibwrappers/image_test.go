package pdflibwrappers_test

import (
	"os"
	"testing"

	"github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers/pdfium_purego"
	"github.com/johbar/text-extraction-service/v4/pkg/pdflibwrappers/poppler_purego"
)

func TestPopplerFindsImage(t *testing.T) {
	_, err := poppler_purego.InitLib("")
	if err != nil {
		t.Fatalf("poppler could not be loaded: %v", err)
	}
	imagePdf := imagePdf()
	d, err := poppler_purego.Load(imagePdf)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	txt, hasImages := d.Text(0)
	if !hasImages {
		t.Error("expected to find images on page")
	}
	if len(txt) > 0 {
		t.Errorf("expected to find no text in PDF, but found '%s'", txt)
	}
}

func TestPdfiumFindsImage(t *testing.T) {
	_, err := pdfium_purego.InitLib("")
	if err != nil {
		t.Fatalf("pdfium could not be loaded: %v", err)
	}
	d, err := pdfium_purego.Load(imagePdf())
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	txt, hasImages := d.Text(0)
	if !hasImages {
		t.Error("expected to find images on page")
	}
	if len(txt) > 0 {
		t.Errorf("expected to find no text in PDF, but found '%s'", txt)
	}
}

func imagePdf() []byte {
	data, err := os.ReadFile("testdata/readme.pdf")
	if err != nil {
		panic(err)
	}
	return data
}
