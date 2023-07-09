package main

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go"
)

var (
	compressor   *zstd.Encoder
	decompressor *zstd.Decoder
)

func init() {
	var err1, err2 error
	compressor, err1 = zstd.NewWriter(nil)
	decompressor, err2 = zstd.NewReader(nil)
	if err1 != nil || err2 != nil {
		log.Fatalf("Could not instantiate (de)compressor: %v, %v", err1, err2)
	}
}

func urlToKey(url string) string {
	k := strings.TrimPrefix(url, "https://")
	k = strings.TrimPrefix(k, "http://")
	return k
}

func getMetaDataFromCache(url string) Metadata {
	key := urlToKey(url)
	entry, err := metadataBucket.Get(key)

	if err != nil {
		if err == nats.ErrKeyNotFound {
			// no log message here
			return nil
		}
		log.Printf("getMetaDataFromCache: %v %s", err, key)
		return nil
	}
	rawValue := entry.Value()
	if rawValue == nil {
		return nil
	}
	metadata := make(map[string]string)
	err = json.Unmarshal(rawValue, &metadata)
	if err != nil {
		log.Printf("%v %s", err, key)
		return nil
	}
	metadata["x-ingested"] = entry.Created().Local().Format(time.RFC3339)
	return metadata
}

func saveMetadataToCache(url string, metadata Metadata) (uint64, error) {
	key := urlToKey(url)
	metadataJson, err := json.Marshal(metadata)
	if err != nil {
		log.Printf("%v %s", err, key)
		return 0, err
	}
	return metadataBucket.Put(key, metadataJson)
}

func getPlaintextFromCache(url string) []byte {
	key := urlToKey(url)
	entry, err := plaintextBucket.Get(key)
	if err != nil {
		if err == nats.ErrKeyNotFound {
			// no log message here
			return nil
		}
		log.Printf("%v %s", err, key)
		return nil
	}
	log.Printf("Retrieved %d compressed bytes from plaintext cache for key %s", len(entry.Value()), key)

	var val []byte
	val, err = decompressBytes(entry.Value())
	if err != nil {
		log.Printf("%v", err)
		return nil
	}
	log.Printf("Decompressed %d bytes to %d bytes", len(entry.Value()), len(val))
	return val
}

func savePlaintextToCache(url string, value []byte) {
	key := urlToKey(url)
	log.Printf("Compressing value for key %s", key)
	uncompressedSize := len(value)
	compressedValue := compressBytes(value)
	ratio := float32(uncompressedSize) / float32(len(compressedValue))
	log.Printf("Compressed %d bytes to %d bytes (%.2fx)", uncompressedSize, len(compressedValue), ratio)
	rev, err := plaintextBucket.Put(key, compressedValue)
	if err != nil {
		log.Printf("ERROR: %v", err)
	}
	log.Printf("Saved %s to Nats. Revision %d", key, rev)
}

func compressBytes(b []byte) []byte {
	dst := make([]byte, 0, len(b))
	return compressor.EncodeAll(b, dst)
}

func decompressBytes(b []byte) ([]byte, error) {
	return decompressor.DecodeAll(b, make([]byte, 0, 4*len(b)))
}
