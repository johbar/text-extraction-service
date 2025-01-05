//go:build embed_pdfium

package pdfium_purego

import (
	"os"
	"os/signal"
)


func ExtractLibpdfium() (string) {
	if len(pdfiumBlob) == 0 {
		println("libpdfium has not been embedded in this build")
		return ""
	}
	f, err := os.CreateTemp("", "libpdfium")
	defer f.Close()
	if err != nil {
		println("Error extracting libpdfium to temp dir:", err)
		return ""
	}
	_, err = f.Write(pdfiumBlob)
	if err != nil {
		println("Error writing libpdfium to temp file:", err)
		return ""
	}
	println("Library extracted to path: ", f.Name())
	// Delete the extracted file before process is terminated
	// We could also delete it, after it has been loaded but then a forked process couldn't use the same file.
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		os.Remove(f.Name())
		println(f.Name(), "deleted")
	}()
	return f.Name()
}
