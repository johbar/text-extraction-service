//go:build !cache_nats_kv

package main

import (
	"io"

	"github.com/nats-io/nats.go"
)

var (
	store nats.ObjectStore
)

func initCache() {
	var err error
	storeConf := &nats.ObjectStoreConfig{Bucket: "texts", Storage: nats.FileStorage}
	store, err = js.CreateObjectStore(storeConf)
	if err != nil {
		panic(err)
	}
	logger.Info("Nats object store initialized.")
}

func getMetadataFromCache(url string) map[string]string {
	info, err := store.GetInfo(url)
	if err == nats.ErrObjectNotFound {
		return nil
	}
	if err != nil {
		logger.Error("Could not get metadata from Nats object store", "url", url)
		return nil
	}
	return info.Metadata
}

func streamPlaintext(url string, w io.Writer) error {
	info, err := store.Get(url)
	if err != nil {
		logger.Error("Error", "err", err)
		return err
	}
	io.Copy(w, info)
	return nil
}

func saveToCache(doc *ExtractedDocument) error {
	m := &nats.ObjectMeta{Metadata: *doc.Metadata, Name: *doc.Url}

	info, err := store.PutBytes(*doc.Url, doc.Text.Bytes())
	if err != nil {
		logger.Error("Could not save text to object store", "err", err.Error())
		return err
	}
	err = store.UpdateMeta(*doc.Url, m)
	if err != nil {
		logger.Error("Could not save metadata to Nats object store", err, err.Error())
	}
	logger.Info("Saved text and metadata in Nats object store bucket", "chunks", info.Chunks, "size", info.Size)
	return err
}
