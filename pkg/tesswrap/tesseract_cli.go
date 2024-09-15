// go:build cli
//go:build !gosseract && !tesseract_wasm && !tesseract_lib

// This is the default implementation
package tesswrap

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
)

func init() {
	_, err := exec.LookPath("tesseract")
	if err != nil {
		Initialized = false
	}
}

func ImageBytesToText(imgBytes []byte) (string, error) {
	r := bytes.NewReader(imgBytes)
	return ImageReaderToText(r)
}

func ImageReaderToText(r io.Reader) (string, error) {
	if r == nil {
		return "", errors.New("reader is nil")
	}
	cmd := exec.Command("tesseract", "-l", Languages, "-", "-")
	cmd.Stdin = r
	result, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(exitErr.Stderr), err
		}
		return "", err
	}
	return string(result), nil
}

func ImageReaderToTextWriter(r io.Reader, w io.Writer) error {
	if r == nil {
		return errors.New("reader is nil")
	}
	cmd := exec.Command("tesseract", "-l", Languages, "-", "-")
	cmd.Stdin = r
	cmd.Stdout = w
	return cmd.Run()
}
