//go:build !gosseract && !tesseract_wasm && !tesseract_lib
// This is the default implementation
package tesswrap

import (
	"bytes"
	"os/exec"
)

func init() {
	_, err := exec.LookPath("tesseract")
	if err != nil {
		Initialized = false
	}
}

func ImageToText(imgBytes []byte) (string, error) {
	cmd := exec.Command("tesseract", "-l", Languages, "-", "-")
	cmd.Stdin = bytes.NewReader(imgBytes)
	result, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(exitErr.Stderr), err
		}
		return "", err
	}
	return string(result), nil
}
