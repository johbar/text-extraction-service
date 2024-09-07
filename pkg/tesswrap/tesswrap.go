/*
Package tesswrap is a rather limited wrapper for Tesseract OCR. It defaults to using the CLI.
Alternative interfaces/implementations can be used by supplying build tags.

This work is in progress.
*/
package tesswrap

var (
// Initialized indicates if this package is usable
	Initialized bool   = true
	Languages   string = "eng+deu"
)