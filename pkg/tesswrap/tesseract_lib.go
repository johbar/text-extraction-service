//go:build tesseract_lib

package tesswrap

import (
	"errors"

	"github.com/raff/go-tesseract"
)

var (
	Version     string
)

func init() {
	Version = tesseract.Version()
	Initialized = true
}

func ImageToText(imgBytes []byte) (string, error) {
	tess := tesseract.BaseAPICreate()
	defer func() {
		tess.Clear()
		// tess.End()
	}()

	if ret := tess.Init3("", Languages); ret != 0 {
		return "", errors.New("could not init tesseract")
	}
	tess.SetDebugVariable("debug_file", "/dev/null")
	tess.SetPageSegMode(tesseract.PSM_AUTO_OSD)
	tess.SetImageBytes(imgBytes)
	txt := tess.GetUTF8Text()
	return txt, nil
}
