//go:build tesseract_lib

package tesswrap

import (
	"errors"
	"io"

	"github.com/raff/go-tesseract"
)



func init() {
	Version = tesseract.Version()
	Initialized = true
}

func ImageReaderToText(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return ImageBytesToText(data)
}

func ImageBytesToText(imgBytes []byte) (string, error) {
	tess := tesseract.BaseAPICreate()
	defer func() {
		tess.Clear()
		tess.End()
	}()

	if ret := tess.Init3("", Languages); ret != 0 {
		return "", errors.New("could not init tesseract")
	}
	tess.SetDebugVariable("debug_file", "/dev/null")
	tess.SetPageSegMode(tesseract.PSM_AUTO_OSD)
	pbytes := tess.SetImageBytes(imgBytes)
	if pbytes == nil {
		return "", errors.New("image could not be processed by Tesseract")
	}
	defer tesseract.FreeImageBytes(pbytes)
	txt := tess.GetUTF8Text()
	return txt, nil
}

func IsTesseractConfigOk() (ok bool, reason string) {
	return true, ""
}