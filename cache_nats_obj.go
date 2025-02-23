//go:build !cache_nop

package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

type ObjectStoreCache struct {
	jetstream.ObjectStore
}

func InitCache(js jetstream.JetStream, conf TesConfig, log slog.Logger) Cache {
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
		log.Error("Error when creating NATS object store", "err", err)
		if conf.FailWithoutJetstream {
			os.Exit(1)
		} else {
			log.Warn("NATS object store could not be initialized and `TES_FAIL_WITHOUT_JS` option is false. Disabling cache.")
			cacheNop = true
			return NopCache{}
		}
	}
	log.Info("NATS object store initialized.")
	return ObjectStoreCache{store}
}

func (store ObjectStoreCache) GetMetadata(url string) (DocumentMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	info, err := store.GetInfo(ctx, url)
	if err == jetstream.ErrObjectNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return info.Metadata, nil
}

func (store ObjectStoreCache) StreamText(url string, w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	info, err := store.Get(ctx, url)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, info)
	return err
}

func (store ObjectStoreCache) Save(doc *ExtractedDocument) (*jetstream.ObjectInfo, error) {
	m := jetstream.ObjectMeta{Metadata: *doc.Metadata, Name: *doc.Url}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r := bytes.NewReader(doc.Text)
	info, err := store.ObjectStore.Put(ctx, m, r)
	if err != nil {
		return info, err
	}
	return info, err
}
