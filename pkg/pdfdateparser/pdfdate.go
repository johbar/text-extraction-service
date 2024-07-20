// Package pdfdateparser provides functions to convert the ModDate and CreationDate fields from PDF metadata to time.Time objects.
package pdfdateparser

import (
	"strings"
	"time"
)

// PdfDateToTime parses the a date/time string from PDF metadata and returns a time.Time object.
func PdfDateToTime(pdfdate string) (time.Time, error) {
	pdfdate, _ = strings.CutPrefix(pdfdate, "D:")
	result, err := time.Parse("20060102150405Z07'00'", pdfdate)
	return result, err
}

// PdfDateToIso returns the PDF date/time as RFC3339 String
func PdfDateToIso(pdfdate string) (string, error) {
	t, err := PdfDateToTime(pdfdate)
	if err != nil {
		return "", err
	}
	return t.Format(time.RFC3339), nil
}
