//go:build tesseract_wasm

// NOTE: Gogoesseract only supports one training data file (language)
package tesswrap

import (
	"context"
	"io"
	"os"

	"github.com/danlock/gogosseract"
)

var (
	tess *gogosseract.Tesseract
)

func init() {
	// TODO handle training data better
	trainingDataFile, _ := os.Open("/usr/share/tesseract-ocr/5/tessdata/Latin.traineddata")
	defer trainingDataFile.Close()
	cfg := gogosseract.Config{
		Language:     Languages,
		TrainingData: trainingDataFile,
	}
	// While Tesseract's output is very useful for debugging, you have the option to silence or redirect it
	cfg.Stderr = io.Discard
	cfg.Stdout = io.Discard
	ctx := context.Background()
	// Compile the Tesseract WASM and run it, loading in the TrainingData and setting any Config Variables provided
	var err error
	tess, err = gogosseract.New(ctx, cfg)
	if err != nil {
		Initialized = false
	}
}

func ImageReaderToText(r io.Reader) (string, error) {
	// Load the image, without parsing it.
	ctx := context.Background()
	err := tess.LoadImage(ctx, r, gogosseract.LoadImageOptions{})
	handleErr(err)
	text, err := tess.GetText(ctx, func(progress int32) {})
	handleErr(err)
	return text, err
}

func ImageReaderToTextWriter(r io.Reader, w io.Writer) error {
	txt, err := ImageReaderToText(r)
	if err == nil {
		w.Write([]byte(txt))
	}
	return err
}

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func IsTesseractConfigOk() (bool, string) {
	return true, ""
}
