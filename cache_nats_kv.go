//go:build cache_nats_kv && !cache_nop

package main

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/bytedance/sonic"
	"github.com/chenzhuoyu/base64x"
	"github.com/nats-io/nats.go/jetstream"
	// "github.com/nats-io/nats.go"
)

var (
	plaintextBucket jetstream.KeyValue
	metadataBucket  jetstream.KeyValue
)

func initCache() {
	var err, findErr error
	plaintextBucket, findErr = js.KeyValue(context.Background(), "plaintexts")
	if findErr != nil {
		if findErr == jetstream.ErrBucketNotFound {
			logger.Info("Nats key value bucket not found. Creating...", "bucket", "plaintexts")
			kvPlainTexts := jetstream.KeyValueConfig{
				Bucket:       "plaintexts",
				MaxValueSize: maxPayload,
				Storage:      jetstream.FileStorage,
				Compression:  true}
			plaintextBucket, err = js.CreateKeyValue(context.Background(), kvPlainTexts)
			if err != nil {
				logger.Error(err.Error())
				os.Exit(1)
			}
		} else {
			logger.Error(findErr.Error())
			os.Exit(1)
		}
	}
	metadataBucket, findErr = js.KeyValue(context.Background(), "metadata")
	if findErr != nil {
		if findErr == jetstream.ErrBucketNotFound {
			logger.Info("Nats key value bucket not found. Creating...", "bucket", "metadata")
			kvMetaConf := jetstream.KeyValueConfig{
				Bucket:       "metadata",
				MaxValueSize: maxPayload,
				Storage:      jetstream.FileStorage}

			metadataBucket, err = js.CreateKeyValue(context.Background(), kvMetaConf)
			if err != nil {
				logger.Error(err.Error())
				os.Exit(1)
			}
		} else {
			logger.Error(findErr.Error())
			os.Exit(1)
		}
	}
	logger.Info("Nats key value buckets initialized.")
}

func urlToKey(url string) string {
	return base64x.JSONStdEncoding.EncodeToString([]byte(url))
}

func getMetadataFromCache(url string) map[string]string {
	key := urlToKey(url)
	entry, err := metadataBucket.Get(context.Background(), key)

	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			return nil
		}
		logger.Error("getMetaDataFromCache", "error", err, "key", key)
		return nil
	}
	rawValue := entry.Value()
	if rawValue == nil {
		return nil
	}
	metadata := make(map[string]string)
	err = sonic.Unmarshal(rawValue, &metadata)
	if err != nil {
		logger.Error("JSON error when loading metadata from cache", "error", err, "key", key)
		return nil
	}
	metadata["x-ingested"] = entry.Created().Local().Format(time.RFC3339)
	return metadata
}

func getPlaintextFromCache(url string) []byte {
	key := urlToKey(url)
	entry, err := plaintextBucket.Get(context.Background(), key)
	if err != nil {
		if err == jetstream.ErrKeyNotFound {
			// no log message here
			return nil
		}
		logger.Error("Error retrieving plaintext from cache", "error", err, "key", key)
		return nil
	}
	return entry.Value()
}

func streamPlaintext(url string, w io.Writer) error {
	data := getPlaintextFromCache(url)
	_, err := w.Write(data)
	return err
}

func saveToCache(doc *ExtractedDocument) error {
	key := urlToKey(*doc.Url)
	metadataJson, err := sonic.Marshal(*doc.Metadata)
	if err != nil {
		logger.Error("JSON error when marshalling metadata", "err", err, "key", key)
		return err
	}
	_, err = metadataBucket.Put(context.Background(), key, metadataJson)
	if err != nil {
		logger.Error("Error putting metadata to KV bucket", "err", err, "key", key)
		return err
	}
	_, err = plaintextBucket.Put(context.Background(), key, doc.Text.Bytes())
	if err != nil {
		logger.Error("Error putting text to KV bucket", "err", err, "key", key)
		return err
	}
	return nil
}
