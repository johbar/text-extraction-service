//go:build cache_nats_kv && !cache_nop

package main

import (
	"io"
	"time"

	"github.com/bytedance/sonic"
	"github.com/chenzhuoyu/base64x"
	"github.com/nats-io/nats.go"
)

var (
	plaintextBucket nats.KeyValue
	metadataBucket  nats.KeyValue
)

func initCache() {
	var err error
	kvPlainTexts := &nats.KeyValueConfig{Bucket: "plaintexts", MaxValueSize: maxPayload, Storage: nats.FileStorage}
	kvMetaConf := &nats.KeyValueConfig{Bucket: "metadata", MaxValueSize: maxPayload, Storage: nats.FileStorage}
	plaintextBucket, err = js.CreateKeyValue(kvPlainTexts)
	if err != nil {
		logger.Error(err.Error())
	}
	metadataBucket, err = js.CreateKeyValue(kvMetaConf)
	if err != nil {
		logger.Error(err.Error())
	}
	logger.Info("Nats key value buckets initialized.")
}

func urlToKey(url string) string {
	return base64x.JSONStdEncoding.EncodeToString([]byte(url))
}

func getMetadataFromCache(url string) map[string]string {
	key := urlToKey(url)
	entry, err := metadataBucket.Get(key)

	if err != nil {
		if err == nats.ErrKeyNotFound {
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
	entry, err := plaintextBucket.Get(key)
	if err != nil {
		if err == nats.ErrKeyNotFound {
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
	_, err = metadataBucket.Put(key, metadataJson)
	if err != nil {
		logger.Error("Error putting metadata to KV bucket", "err", err, "key", key)
		return err
	}
	_, err = plaintextBucket.Put(key, doc.Text.Bytes())
	if err != nil {
		logger.Error("Error putting text to KV bucket", "err", err, "key", key)
		return err
	}
	return nil
}
