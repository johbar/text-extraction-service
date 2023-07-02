package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go"
)

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

	var buf bytes.Buffer
	val := bytes.NewBuffer(entry.Value())

	err = decompress(val, &buf)
	if err != nil {
		log.Printf("%v", err)
		return nil
	}
	return buf.Bytes()
}

func savePlaintextToCache(url string, value bytes.Buffer) {
	key := urlToKey(url)
	log.Printf("Compressing value for key %s", key)
	var compressedValue bytes.Buffer
	uncompressedSize := value.Len()
	err := compress(&value, &compressedValue)
	if err != nil {
		log.Printf("%v", err)
	}
	ratio := float32(uncompressedSize) / float32(compressedValue.Len())
	log.Printf("Compressed %d bytes to %d bytes (%.2fx)", uncompressedSize, compressedValue.Len(), ratio)
	rev, err := plaintextBucket.Put(key, compressedValue.Bytes())
	if err != nil {
		log.Printf("ERROR in saveToNatsZstd: %v", err)
	}
	log.Printf("Saved %d bytes to Nats. Revision %d", compressedValue.Len(), rev)
}

func compress(in io.Reader, out io.Writer) error {
	enc, err := zstd.NewWriter(out)
	if err != nil {
		return err
	}
	_, err = io.Copy(enc, in)
	if err != nil {
		enc.Close()
		return err
	}
	return enc.Close()
}

func decompress(in io.Reader, out io.Writer) error {
	d, err := zstd.NewReader(in)
	if err != nil {
		return err
	}
	defer d.Close()

	// Copy content...
	_, err = io.Copy(out, d)
	return err
}
