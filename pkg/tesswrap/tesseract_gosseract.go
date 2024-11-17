//go:build gosseract

package tesswrap

import (
	"io"

	"github.com/otiai10/gosseract/v2"
)

func init() {
	Version = gosseract.Version()
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
	goss := gosseract.NewClient()
	goss.Trim = true
	defer goss.Close()
	// This option doesn't seem to work
	goss.SetPageSegMode(gosseract.PSM_AUTO_OSD)
	goss.DisableOutput()
	goss.SetLanguage(Languages)
	goss.SetImageFromBytes(imgBytes)
	txt, err := goss.Text()
	return txt, err
}
