//go:build !cache_nats_kv && !cache_nop

package main

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

var (
	store jetstream.ObjectStore
)

func initCache(bucket string) {
	var err error
	store, err = js.ObjectStore(context.Background(), bucket)
	if err != nil {
		logger.Info("Error when connecting NATS object store", "err", err)
		store, err = js.CreateObjectStore(
			context.Background(),
			jetstream.ObjectStoreConfig{
				Storage:     jetstream.FileStorage,
				Bucket:      bucket,
				Compression: true,
			})
		if err != nil {
			logger.Error("Error when creating NATS object store", "err", err)
			os.Exit(1)
		}
	} else {
		logger.Info("NATS object store found", "bucket", bucket)
	}

	logger.Info("NATS object store initialized.")
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := store.PutBytes(ctx, *doc.Url, doc.Text.Bytes())
	if err != nil {
		logger.Error("Could not save text to object store", "err", err)
		return err
	}
	err = store.UpdateMeta(context.Background(), *doc.Url, m)
	if err != nil {
		logger.Error("Could not save metadata to Nats object store", err, err.Error())
	}
	logger.Info("Saved text and metadata in Nats object store bucket", "chunks", info.Chunks, "size", info.Size)
	return err
}
