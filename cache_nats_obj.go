//go:build !cache_nats_kv && !cache_nop

package main

import (
	"context"
	"io"
	"os"

	"github.com/nats-io/nats.go/jetstream"
)

var (
	store jetstream.ObjectStore
)

func initCache() {
	var err error
	store, err = js.ObjectStore(context.Background(), "plaintexts")
	if err != nil {
		logger.Info("Error when connecting Nats object store", "err", err)
		store, err = js.CreateObjectStore(context.Background(),
			jetstream.ObjectStoreConfig{
				Storage: jetstream.FileStorage,
				Bucket:  "plaintexts"})
		if err != nil {
			logger.Error("Error when creating Nats object store", "err", err)
			os.Exit(1)
		}
	} else {
		logger.Info("Nats object store found", "bucket", "plaintexts")
	}
	logger.Info("Nats object store initialized.")
}

func getMetadataFromCache(url string) DocumentMetadata {
	info, err := store.GetInfo(context.Background(), url)
	if err == jetstream.ErrObjectNotFound {
		return nil
	}
	if err != nil {
		logger.Error("Could not get metadata from Nats object store", "url", url, "err", err)
		return nil
	}
	return info.Metadata
}

func streamPlaintext(url string, w io.Writer) error {
	info, err := store.Get(context.Background(), url)
	if err != nil {
		logger.Error("Error", "err", err)
		return err
	}
	io.Copy(w, info)
	return nil
}

func saveToCache(doc *ExtractedDocument) error {
	m := jetstream.ObjectMeta{Metadata: *doc.Metadata, Name: *doc.Url}

	info, err := store.PutBytes(context.Background(), *doc.Url, doc.Text.Bytes())
	if err != nil {
		logger.Error("Could not save text to object store", "err", err.Error())
		return err
	}
	err = store.UpdateMeta(context.Background(), *doc.Url, m)
	if err != nil {
		logger.Error("Could not save metadata to Nats object store", err, err.Error())
	}
	logger.Info("Saved text and metadata in Nats object store bucket", "chunks", info.Chunks, "size", info.Size)
	return err
}
