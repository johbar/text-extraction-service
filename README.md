# text-extraction-service

A simple Go service for extracting textual content from PDF, RTF and legacy MS Word (.doc) documents.

## Status

This started as an exercise in using Golang.
The use case is processing binary documents for search machine indexation.

It optionally embeds [NATS.io](https://nats.io/) JetStream as an object store that acts as a persistent cache for extracted content.

The RegEx-based RTF parser is rather inefficient.

Apache [Tika](https://tika.apache.org/) is definitively a more versatile and mature solution to be considered.

## Features

- Support for PDFs, RTFs and legacy MS Word (.doc) files
- Dehyphenation of extracted text, specifically for German
- NATS can be embedded or run externally (e.g. as a cluster)
- NATS microservice interface supported

## Setup

By default Poppler is used for PDFs. It is needed as dependency which can be installed on Debian via `apt-get install libpoppler-glib-dev`.

## Build locally

```sh
# Omit a large, yet unused dependency of Gin
go build -tags nomsgpack
```

## Use MuPDF instead of Poppler via build tag

```sh
go build -tags mupdf,nomsgpack
```

## MuPDF or Poppler?

MuPDF is very fast for small documents.
Poppler is rather slow but faster and more memory-efficient for large documents.

In my experience Poppler offers the overall best quality when performing text extraction.

## Build container images

The repo includes two Containerfiles for building minimal Alpine-based images in a multi-stage style.

```sh
# Use a volume to speed up subsequent builds—remove the need to re-download and re-compile all dependencies
mkdir --mode 777 --parents /tmp/cache

# MuPDF-based:
podman build --pull . -f Containerfile.mupdf-alpine -t tes-mupdf:alpine --volume /tmp/cache:/tmp

# Poppler-based:
podman build --pull . -f Containerfile.poppler-alpine -t tes-poppler:alpine --volume /tmp/cache:/tmp
```

## Run containers

```sh
# MuPDF based, using a volume for Nats JetStream storage
podman run --rm -it -v nats:/tmp/nats -p 8080:8080 -p 4222:4222 tes-mupdf:alpine

# poppler based, using a volume for Nats JetStream storage
podman run --rm -it -v nats:/tmp/nats -p 8080:8080 -p 4222:4222 tes-poppler:alpine
```

## Config

Configuration happens trough enviroment Variables only.

| Environment Variable  | Description                                                                   |
| --------------------- | ----------------------------------------------------------------------------- |
| `TES_BUCKET`          | Name of the object store bucket in NATS to use for caching. Default: `TES_PLAINTEXTS` |
| `TES_REPLICAS`        | Replication factor for object store bucket in external NATS cluster           |
| `TES_EXPOSE_NATS`     | Wether to allow connections to the embedded NATS server (bool)                |
| `TES_NATS_HOST`       | Listen host/IP of embedded NATS server when `TES_EXPOSE_NATS`is `true`. Default: `localhost` |
| `TES_NATS_PORT`       | Listen Port (TCP) of embedded NATS server when `TES_EXPOSE_NATS`is `true`. Default: `4222` |
| `TES_NATS_STORE_DIR`  | Storage path for the embedded NATS server. Default: `/tmp/nats`               |
| `TES_MAX_PAYLOAD`     | Max message payload of embedded NATS server. Default: `8 MiB`                 |
| `TES_NATS_URL`        | URL of external NATS server/cluster. If this is set, no embedded NATS server is started |
| `TES_FAIL_WITHOUT_JS` | If enabled the service exits when JetStream support of the external NATS server/cluster can not be confirmed. Default: `true` |
| `TES_NATS_TIMEOUT`    | Connection timeout as a `time.Duration` string. Default: `15s`                |
| `TES_NATS_CONNECT_RETRIES` | Number of times a connection to an external NATS server/cluster and to JetStream is being tried. Default: `10` |
| `TES_HOST_PORT`       | Listen adress of HTTP server. Default: `:8080` (same as `0.0.0.0:8080`)       |
| `TES_NO_HTTP`         | If `true` and `TES_EXPOSE_NATS` is `true`, too, no HTTP server is started     |

## Usage

TBD
