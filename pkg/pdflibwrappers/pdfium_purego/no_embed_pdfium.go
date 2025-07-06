//go:build !embed_pdfium

package pdfium_purego

import "errors"

const PdfiumEmbedded = false

func ExtractLibpdfium() (string, error) {
	return "", errors.New("pdfium is not embedded in this build")
}
