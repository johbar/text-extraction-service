//go:build cache_nop

package main

import "io"

func init() {
	cacheNop = true
}

func initCache(bucket String, replicas int) {
}

func getMetadataFromCache(url string) map[string]string {
	return nil
}

func streamPlaintext(url string, w io.Writer) error {
	return nil
}

func saveToCache(doc *ExtractedDocument) error {
	return nil
}
