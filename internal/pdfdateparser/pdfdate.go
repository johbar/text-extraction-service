// Package pdfdateparser provides functions to convert the ModDate and CreationDate fields from PDF metadata to time.Time objects.
package pdfdateparser

import (
	"fmt"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PdfDateToTime parses the a date/time string from PDF metadata and returns a time.Time object.
func PdfDateToTime(pdfdate string) (time.Time, error) {
	result, ok := types.DateTime(pdfdate, true)
	var err error = nil
	if !ok {
		err = fmt.Errorf("date %s could not be parsed", pdfdate)
	}
	return result, err
}

// PdfDateToIso returns the PDF date/time as RFC3339 string.
// Returns an empty string in case error is not nil.
func PdfDateToIso(pdfdate string) string {
	if pdfdate == "" {
		return ""
	}
	t, err := PdfDateToTime(pdfdate)
	if err != nil {
		return ""
	}
	return t.Format(time.RFC3339)
}
