//go:build !cache_nats_kv && !cache_nop

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type ObjectStoreCache struct {
	jetstream.ObjectStore
}

func InitCache(js jetstream.JetStream, conf TesConfig) Cache {
	var err error
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	store, err := js.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{
		Storage:     jetstream.FileStorage,
		Bucket:      conf.Bucket,
		Compression: true,
		Replicas:    conf.Replicas,
	})
	if err != nil {
		logger.Error("Error when creating NATS object store", "err", err)
		if conf.FailWithoutJetstream {
			os.Exit(1)
		} else {
			logger.Warn("NATS object store could not be initialized and " + confFailWithoutJs + "option is false. Disabling cache.")
			cacheNop = true
			return NopCache{}
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	info, err := store.Get(ctx, url)
	if err != nil {
		logger.Error("Could not receive text from NATS object store", "url", url, "err", err)
		return err
	}
	io.Copy(w, info)
	return nil
}

func (store ObjectStoreCache) Save(doc *ExtractedDocument) error {
	m := jetstream.ObjectMeta{Metadata: *doc.Metadata, Name: *doc.Url}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r := bytes.NewReader(doc.Text)
	info, err := store.ObjectStore.Put(ctx, m, r)
	if err != nil {
		logger.Error("Could not save text and metadata to NATS object store", "err", err)
		return err
	}
	logger.Info("Saved text and metadata in NATS object store bucket", "url", *doc.Url, "chunks", info.Chunks, "size", info.Size)
	return err
}
