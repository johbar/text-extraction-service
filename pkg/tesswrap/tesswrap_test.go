package tesswrap

import (
	"os"
	"testing"
)

func TestImageToText(t *testing.T) {
	if !Initialized {
		t.Log("Tesseract not available")
		return
	}
	Languages = "eng"
	f, err := os.Open("testdata/readme.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	txt, err := ImageReaderToText(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(txt) == 0 {
		t.Fatal("zero-length content")
	}
	t.Log(txt)
}
