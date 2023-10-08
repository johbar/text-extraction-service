package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/chenzhuoyu/base64x"
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
	return base64x.JSONStdEncoding.EncodeToString([]byte(url))
}

func getMetadataFromCache(url string) map[string]string {
	key := urlToKey(url)
	entry, err := metadataBucket.Get(key)

	if err != nil {
		if err == nats.ErrKeyNotFound {
			// no log message here
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
	err = json.Unmarshal(rawValue, &metadata)
	if err != nil {
		logger.Error("JSON error when loading metadata from cache", "error", err, "key", key)
		return nil
	}
	metadata["x-ingested"] = entry.Created().Local().Format(time.RFC3339)
	return metadata
}

func saveMetadataToCache(data ExtractedDocument) (uint64, error) {
	key := urlToKey(*data.Url)
	metadataJson, err := json.Marshal(*data.Metadata)
	if err != nil {
		logger.Error("JSON error when saving metadata to cache", "error", err, "key", key)
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
		logger.Error("Error retrieving plaintext from cache", "error", err, "key", key)
		return nil
	}

	var val []byte
	val, err = decompressBytes(entry.Value())
	if err != nil {
		log.Printf("%v", err)
		return nil
	}
	logger.Debug("Decompressed plaintext", "compressed_bytes", len(entry.Value()), "decompressed_bytes", len(val))
	return val
}

func savePlaintextToCache(doc *ExtractedDocument) (revision uint64, err error) {
	url := *doc.Url
	text := doc.Text.Bytes()
	key := urlToKey(url)
	// log.Printf("Compressing value for key %s", key)
	uncompressedSize := len(text)
	value := compressBytes(text)
	ratio := float32(uncompressedSize) / float32(len(value))
	logger.Debug("Compressed plaintext", "uncompressed_bytes", uncompressedSize, "compressed_bytes", len(value), "ratio", ratio)
	// log.Printf("Compressed %d bytes to %d bytes (%.2fx)", uncompressedSize, len(value), ratio)
	return plaintextBucket.Put(key, value)
}

func compressBytes(b []byte) []byte {
	dst := make([]byte, 0, len(b))
	return compressor.EncodeAll(b, dst)
}

func decompressBytes(b []byte) ([]byte, error) {
	return decompressor.DecodeAll(b, make([]byte, 0, 4*len(b)))
}
