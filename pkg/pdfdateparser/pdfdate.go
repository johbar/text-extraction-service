// Package pdfdateparser provides functions to convert the ModDate and CreationDate fields from PDF metadata to time.Time objects.
package pdfdateparser

import (
	"strings"
	"time"
)

// PdfDateToTime parses the a date/time string from PDF metadata and returns a time.Time object.
func PdfDateToTime(pdfdate string) (time.Time, error) {
	patterns := []string{"20060102150405Z07", "20060102150405Z07'00'", "20060102150405"}
	pdfdate, _ = strings.CutPrefix(pdfdate, "D:")
	var result time.Time
	var err error
	for _, pattern := range patterns {
		result, err = time.Parse(pattern, pdfdate)
		if err == nil {
			return result, err
		}
	}
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
