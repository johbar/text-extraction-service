//go:build embed_pdfium

package pdfium_purego

import (
	_ "embed"
)

var (
	//go:embed lib/libpdfium.so
	pdfiumBlob []byte
)
