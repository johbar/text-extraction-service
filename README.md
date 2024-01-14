# text-extraction-service

A simple Go service for extracting textual content from PDF, RTF and legacy MS Word (.doc) documents.

## Status

This started as an exercise in using Golang.
The use case is processing binary documents for search machine indexation.

It embeds [Nats](https://nats.io/) as a key-value-/object store that acts as a persistent cache for extracted content.

The RegEx-based RTF parser is rather inefficient.

## Setup

By default Poppler is used for PDFs.
Install on Debian via `apt-get install libpoppler-glib-dev`.

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

TBD

## Usage

TBD