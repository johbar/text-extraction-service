package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	config "github.com/johbar/text-extraction-service/v2/internal/config"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type ObjectStoreCache struct {
	jetstream.ObjectStore
	nc *nats.Conn
	js jetstream.JetStream
}

func New(conf config.TesConfig, log *slog.Logger, nc *nats.Conn) (*ObjectStoreCache, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if nc == nil {
		return nil, errors.New("no connection to NATS")
	}
	js, err := setupJetstream(conf, nc, log)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	store, err := js.CreateOrUpdateObjectStore(ctx, jetstream.ObjectStoreConfig{
		Storage:     jetstream.FileStorage,
		Bucket:      conf.Bucket,
		Compression: true,
		Replicas:    conf.Replicas,
	})
	if err != nil {
		log.Error("Creating NATS object store failed", "err", err)
		if conf.FailWithoutJetstream {
			return nil, fmt.Errorf("initializing NATS object store: %w", err)
		}
	}
	log.Info("NATS object store initialized.")
	return &ObjectStoreCache{store, nc, js}, nil
}

func setupJetstream(conf config.TesConfig, nc *nats.Conn, log *slog.Logger) (jetstream.JetStream, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		log.Error("FATAL: Error when initializing NATS JetStream", "err", err.Error())
		return nil, err
	}

	for attempts := 0; attempts <= conf.NatsConnectRetries; attempts++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = js.AccountInfo(ctx)
		if err != nil {
			if errors.Is(err, jetstream.ErrJetStreamNotEnabled) || errors.Is(err, jetstream.ErrJetStreamNotEnabledForAccount) {
				return nil, err
			}
			log.Error("NATS JetStream check failed. Is JetStream enabled in external NATS server(s)?",
				"err", err,
				"count", attempts,
				"maxRetries", conf.NatsConnectRetries)
			time.Sleep(time.Second)
		} else {
			return js, nil
		}
	}
	return nil, fmt.Errorf("retry count exceeded: %w", err)
}

func (store ObjectStoreCache) GetMetadata(url string) (DocumentMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	info, err := store.GetInfo(ctx, url)
	if err == jetstream.ErrObjectNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("retrieving object metadata for %s: %w", url, err)
	}
	return info.Metadata, nil
}

func (store ObjectStoreCache) StreamText(url string, w io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	info, err := store.Get(ctx, url)
	if err != nil {
		return fmt.Errorf("retrieving object %s from object store: %w", err)
	}
	_, err = io.Copy(w, info)
	return err
}

func (store ObjectStoreCache) Save(doc ExtractedDocument) (*jetstream.ObjectInfo, error) {
	m := jetstream.ObjectMeta{Metadata: *doc.Metadata, Name: *doc.Url}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r := bytes.NewReader(doc.Text)
	info, err := store.ObjectStore.Put(ctx, m, r)
	return info, err
}
