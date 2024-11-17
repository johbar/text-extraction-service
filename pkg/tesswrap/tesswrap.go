/*
Package tesswrap is a rather limited wrapper for Tesseract OCR v5.
It defaults to using the CLI.
Alternative interfaces/implementations can be used by supplying build tags.

This is work in progress.
*/
package tesswrap

var (
	// Initialized indicates if this package is usable
	Initialized bool = true
	// Languages tesseract shall take into consideration when performing OCR
	Languages string = "Latin+osd"
	Version   string
)
