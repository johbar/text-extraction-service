package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// PrintMetadataAndTextToStdout prints a file's metadata (as JSON) on the first line, followed by the file's text content.
// The file can be local or remote (http/https). When url is "-", the file will be read from Stdin
func PrintMetadataAndTextToStdout(url string) {
	var doc Document
	var stream io.ReadCloser
	if strings.HasPrefix(url, "http") {
		resp, err := http.Get(url)
		if err != nil {
			os.Exit(1)
		}
		if resp.StatusCode >= 400 {
			logger.Error("HTTP error", "url", url, "err", err)
			os.Exit(1)
		}
		stream = resp.Body
	} else {
		if url == "-" {
			stream = os.Stdin
		} else {
			f, err := os.Open(url)
			if err != nil {
				logger.Error("Could not open file", "err", err)
				os.Exit(1)
			}
			defer f.Close()
			stream = f
		}
	}
	doc, err := NewDocFromStream(stream)
	if err != nil {
		logger.Error("Could not process document", "url", url, "err", err)
		os.Exit(2)
	}
	meta, _ := json.Marshal(doc.MetadataMap())
	os.Stdout.Write(meta)
	fmt.Println()
	doc.StreamText(os.Stdout)
	fmt.Println()
}
