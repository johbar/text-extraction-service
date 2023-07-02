# text-extraction-service

A simple Golang service for extracting textual content from PDF documents.

## Status

This is an exercise in using Golang, probably unsuitable for any real-world usage.

It embeds [Nats](https://nats.io/) as a key-value-store that acts as a persistent cache for extracted content.

## Setup

Poppler is needed for PDFs: `apt-get install libpoppler-glib-dev libcairo2-dev`


