# text-extraction-service

A simple Golang service for extracting textual content from PDF documents.

## Status

This is an exercise in using Golang, probably unsuitable for any real-world usage.

It embeds [Nats](https://nats.io/) as a key-value-store that acts as a persistent cache for extracted content.

## Setup

By default poppler is used for PDFs.
Install on Debian via `apt-get install libpoppler-glib-dev`.

## Use MuPDF instead of Poppler via build tag

```sh
go build -tags mupdf
```

## MuPDF or Poppler?

MuPDF is very fast for small documents. Poppler is rather slow but faster and more memory-efficient for large documents.

## Container build

The repo includes two Containerfiles for building minimal Alpine-based images.

```sh
# Use volumes to speed up subsequent builds -- remove the need to re-download and re-compile all dependencies:
podman build . -f Containerfile.mupdf-alpine -t tes-mupdf:alpine-minimal --volume /tmp/buildcache:/go --volume /tmp/cache:/.cache/

# for Poppler-based build:
podman build . -f Containerfile.poppler-alpine -t tes-poppler:alpine-minimal --volume /tmp/buildcache:/go --volume /tmp/cache:/.cache/
```

## Config

TBD

## Usage

TBD