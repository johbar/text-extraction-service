//go:build gosseract

package tesswrap

import (
	"github.com/otiai10/gosseract/v2"
)

var (
	Version string
)

func init() {
	Version = gosseract.Version()
	Initialized = true
}

func ImageToText(imgBytes []byte) (string, error) {
	goss := gosseract.NewClient()

	defer goss.Close()
	// This option doesn't seem to work
	goss.SetPageSegMode(gosseract.PSM_AUTO_OSD)
	goss.DisableOutput()
	goss.SetLanguage(Languages)
	goss.SetImageFromBytes(imgBytes)
	goss.Trim = true
	txt, err := goss.Text()
	return txt, err
}
