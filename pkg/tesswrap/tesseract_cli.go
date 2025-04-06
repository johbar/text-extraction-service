//go:build !tesseract_pure

// This is the default implementation
package tesswrap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strings"
)

var LangsAvailable []string

func init() {
	if _, err := exec.LookPath("tesseract"); err != nil {
		Initialized = false
	} else {
		LangsAvailable = listLangs()
	}
}

func listLangs() []string {
	cmd := exec.Command("tesseract", "--list-langs")
	output, err := cmd.Output()
	if err != nil {
		return []string{}
	}
	outputLines := strings.Split(string(output), "\n")
	outputLen := len(outputLines) - 1
	if outputLen > 1 {
		// first line is a heading
		// last element is empty due to trailing newline
		return outputLines[1 : outputLen-1]
	} else {
		return []string{}
	}
}

// TesseractConfigOk returns true and an empty string, if Tessearct is installed in PATH
// and the configured languages have trained data models.
// If not, false and a reason phrase reporting the first missing language file are returned.
func TesseractConfigOk() (ok bool, reason string) {
	LangSlice := strings.Split(Languages, "+")
	for _, elem := range LangSlice {
		if !slices.Contains(LangsAvailable, elem) {
			return false, fmt.Sprintf("'%s' is not among the installed languages %v", elem, LangsAvailable)
		}
	}
	return Initialized, ""
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

func ImageBytesToWriter(data []byte, w io.Writer) error {
	r := bytes.NewReader(data)
	return ImageReaderToWriter(r, w)
}

func ImageReaderToWriter(r io.Reader, w io.Writer) error {
	if r == nil {
		return errors.New("reader is nil")
	}
	cmd := exec.Command("tesseract", "-l", Languages, "-", "-")
	cmd.Stdin = r
	cmd.Stdout = w
	return cmd.Run()
}

func ImageToWriter(path string, w io.Writer) error {
	cmd := exec.Command("tesseract", "-l", Languages, path, "-")
	cmd.Stdout = w
	return cmd.Run()
}
