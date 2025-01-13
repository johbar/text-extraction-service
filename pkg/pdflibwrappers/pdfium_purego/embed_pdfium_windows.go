//go:build embed_pdfium

package pdfium_purego

import (
	_ "embed"
)

var (
	//go:embed lib/pdfium.dll
	pdfiumBlob []byte
)
