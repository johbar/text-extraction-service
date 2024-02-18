//go:build !cache_nats_kv && !cache_nop

package main

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type Cache interface {
	// Init(js jetstream.JetStream, bucket string, replicas int) Cache
	GetMetadata(url string) DocumentMetadata
	StreamText(url string, w io.Writer) error
	Save(doc *ExtractedDocument) error
}

type ObjectStoreCache struct {
	jetstream.ObjectStore
}



func InitCache(js jetstream.JetStream, bucket string, replicas int) Cache {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := js.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{
		Storage:     jetstream.FileStorage,
		Bucket:      bucket,
		Compression: true,
		Replicas:    replicas,
	})
	if err != nil {
		logger.Error("Error when creating NATS object store", "err", err)
		os.Exit(1)
	}
	logger.Info("NATS object store initialized.")
	return ObjectStoreCache{store}
}

func (store ObjectStoreCache) GetMetadata(url string) DocumentMetadata {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	info, err := store.GetInfo(ctx, url)
	if err == jetstream.ErrObjectNotFound {
		return nil
	}
	if err != nil {
		logger.Error("Could not get metadata from NATS object store", "url", url, "err", err)
		return nil
	}
	return info.Metadata
}

func (store ObjectStoreCache) StreamText(url string, w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	info, err := store.Get(ctx, url)
	if err != nil {
		logger.Error("Error", "err", err)
		return err
	}
	io.Copy(w, info)
	return nil
}

func (store ObjectStoreCache) Save(doc *ExtractedDocument) error {
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
		logger.Error("Could not save metadata to NATS object store", err, err.Error())
	}
	logger.Info("Saved text and metadata in NATS object store bucket", "chunks", info.Chunks, "size", info.Size)
	return err
}
