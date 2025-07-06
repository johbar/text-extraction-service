//go:build embed_pdfium

package pdfium_purego

import (
	"errors"
	"os"
)

const PdfiumEmbedded = true

func ExtractLibpdfium() (string, error) {

	if len(pdfiumBlob) == 0 {
		return "", errors.New("extraction of libpdfium has been requested, but it is not embedded in this build")
	}
	f, err := os.CreateTemp("", "libpdfium*"+libExtension)
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.Write(pdfiumBlob)
	return f.Name(), err
}
