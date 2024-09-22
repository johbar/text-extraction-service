# text-extraction-service

TES is a simple Go service for extracting and storing textual content from PDF, RTF and legacy MS Word (.doc) documents.

## Status

This started as an exercise in using Golang and cgo.
But it is about to be used in production (at least for PDFs).
The use case is the fast processing of binary documents for repeated search machine indexation.

The RegEx-based RTF parser is rather inefficient.

Apache [Tika](https://tika.apache.org/) is definitively a more versatile and mature solution to be considered.

## Features

- Support for PDFs, RTFs and legacy MS Word (.doc) files
- Three C/C++ PDF engine implementations available via build tags
    - Google Chromium's [PDFium](https://pdfium.googlesource.com/pdfium/) via [go-pdfium](https://github.com/klippa-app/go-pdfium)
    - Free Desktops [Poppler lib](https://poppler.freedesktop.org/) via [go-poppler](https://github.com/johbar/go-poppler)
    - Artifex' [MuPDF](https://mupdf.com/) via [go-fitz](https://github.com/gen2brain/go-fitz)
- Optional Dehyphenation of extracted text, specifically for German
- Extraction of document metadata (title, author, creation date etc)
- Store extracted text and metadata in NATS for faster retrieval
- NATS can be embedded or run externally (e.g. as a cluster)
- Support for NATS microservice interface
- (Experimental) Optical character recognition by [Tesseract OCR](https://github.com/tesseract-ocr/) (useful for images containing text and scanned PDFs)

## Unsupported

- Processing local files with the `file:` transport
- Processing password protected files
- Processing files from web servers that require authentication of any kind (cookie, header, referral, user agent etc)
- A lot of common document formats, including odt, docx, html, xml

## License

This service inherits the Open Source license of the PDF lib used to built it:

- PDFium/go-pdfium: [Apache-2](https://pdfium.googlesource.com/pdfium/+/master/LICENSE), [MIT](https://github.com/klippa-app/go-pdfium/blob/main/LICENSE)
- Poppler/go-poppler: GPL-2.0
- MuPDF/go-fitz: AGPL-3.0 (commercial license available)

That's the reason why there is no default implementation anymore.
You always need to supply a build tag.

## Dev Setup - Building TES

Depending on the PDF engine you choose (see below for comparison) you need dependencies being to be installed in dev/build environment.

In any case you need a recent Go SDK (v1.21+) and (with the exception of PDFium-WASM) a C compiler toolchain.

All instructions supplied here suppose a unix-like environment.

### PDFium

Follow the instructions in [go-pdfium](https://github.com/klippa-app/go-pdfium):

- Download the PDFium binaries and header files or compile the lib yourself.
- Create a PKG config file (preferably in `/usr/local/lib/pkgconfig/pdfium.pc`).
- Set `LD_LIBRARY_PATH` and `PKG_CONFIG_PATH` if needed.

### PDFium Webassembly

If you want to test PDFium without the cgo hassle use the build tag `pdfium_wasm`.
This will decrease the speed of text extraction and will make the service boot slower.

### Poppler

Install dependencies on Debian based systems via `apt-get install libpoppler-glib-dev`.

### MuPDF

Everything you need is included in the `go-fitz` wrapper (header files, binaries).

## Build locally

To build the service just run `go build` with one of these `tags`:

- `pdfium`
- `pdfium_wasm`
- `poppler`
- `mupdf`

I recommend supplying the tag `nomsgpack` as well, shrinking the build.
See [Gin docs](https://github.com/gin-gonic/gin/blob/master/docs/doc.md#build-without-msgpack-rendering-feature).

Examples:

```sh
go build -tags nomsgpack,pdfium -o tes-pdfium
go build -tags nomsgpack,poppler -o tes-poppler
go build -tags nomsgpack,mupdf -o tes-mupdf
```

If you don't need the NATS based cache additionally supply the build tag `cache_nop`.

## OCR (experimental)

If you want to process image files or scanned PDFs TES got you covered.
All you need to do is:

1. Install Tesseract, on Debian/Ubuntu run `apt install tesseract-ocr`
2. Install any language model file you need.
    1. Run `apt install tesseract-ocr-script-latn` for a model that supports multiple languages with latin script (rather large)
    2. Or run `apt install tesseract-ocr-deu` for the german model (smaller but specific)
3. Configure TES to pass on your language preference to Tesseract by setting the environment variable, e.g. `TES_TESSERACT_LANGS=Latin+osd` when running TES.
4. Optionally set `TESS_PREFIX` if you have installed the language models in a custom path.

If there is no text found on a PDF page and Tesseract is available TES will look for images on that page.
It will then extract these images in-memory and pipe them to the Tesseract CLI.
The output is then streamed back to TES or rather the client (after dehyphenation and compaction).
This means: If there a mixture of text and image, no OCR is performed.

NOTE: OCR is an expensive process and can take a lot of time and resources.
It is not fully accurate.

NOTE: This feature is not well tested and thus considered experimental.

##  PDFium, MuPDF or Poppler?

Concerning the quality of text extracted by theses libs in my experience *Poppler* and *PDFium* are better than *MuPDF*.
But complicated as the Portable Document Format is there are a lot of edge cases one lib handles better than the other–and some where neither can do right.

Regarding speed with ordinary (rather small) files *PDFium* and *MuPDF* are mostly astride.

Some other aspects:

|                              | PDFium                   | Poppler            | MuPDF                   |
|------------------------------|--------------------------|--------------------|-------------------------|
| License                      | ✅ permissive             | ⚠️ Copyleft        | ⚠️ Copyleft              |
| Performance with small files | ✅ good                   | ❌ bad              | ✅ good                  |
| Performance with large files | ✅ good                   | 🚀 best            | ❌ bad                   |
| Memory consumption           | ❌ high with large files¹ | ✅ consistently low | ❌ high with large files |
| Available from Linux sources
(deb, rpm, apk)                | ❌ no¹ | ✅ headers & lib | ✅ headers & static lib
| Multi-threaded               | ❌ no²                     | ✅ yes        | ✅ yes        |

¹ At runtime you can use the LibreOffice build of *PDFium*, `libpdfiumlo.so` from the Debian package `libreoffice-core-nogui`.
Using this lib instead of [bblanchon/pdfium-binaries](https://github.com/bblanchon/pdfium-binaries) performance drops a bit (maybe 10%), but in turn memory consumption with large files decreases a lot.
See [Containerfile](Containerfile.pdfiumlo-ubuntu) on how to use this shared lib.

² *PDFium* is not thread safe.
For that reason in single-threaded mode `go-pdfium` uses a lock to protect the lib instance against concurrent access.
TES sticks to that mode instead of the alternatives (Webassembly or multiprocessing via gRPC)
because they are bad for performance.
In my tests with `curl --parallel` and ~100 Files it was still faster than the multi-threaded WASM version.

## Build container images

This repo includes a bunch of Containerfiles for building images in a multi-stage style.

I use Podman but everything should work with Docker as well.

Ubuntu refers to Ubuntu 22.04 (*Jammy*).

```sh
# Use a volume to speed up subsequent builds—remove the need to re-download and re-compile all dependencies
mkdir --mode 777 --parents /tmp/cache

# MuPDF on Alpine:
podman build --pull . -f Containerfile.mupdf-alpine -t tes:mupdf-alpine --volume /tmp/cache:/tmp

# Poppler on Alpine:
podman build --pull . -f Containerfile.poppler-alpine -t tes:poppler-alpine --volume /tmp/cache:/tmp

# Poppler on Ubuntu:
podman build --pull . -f Containerfile.poppler-ubuntu -t tes:poppler-ubuntu --volume /tmp/cache:/tmp

# PDFium on Ubuntu:
podman build --pull . -f Containerfile.pdfium-ubuntu -t tes:pdfium-ubuntu --volume /tmp/cache:/tmp

# PDFium from Libreoffice on Ubuntu
podman build --pull . -f Containerfile.pdfiumlo-ubuntu -t tes:pdfiumlo-ubuntu --volume /tmp/cache:/tmp

# PDFium + Tesseract (Latin/multi-lang) on Ubuntu
podman build --pull . -f Containerfile.pdfiumlo-tesseract-latin-ubuntu -t tes:pdfiumlo-ocr-ubuntu --volume /tmp/cache:/tmp
```

## Run TES in a container

Examples

```sh
# MuPDF based, using a volume for NATS JetStream storage
podman run --rm -it -v nats:/tmp/nats -p 8080:8080 -p 4222:4222 tes:mupdf-alpine

# Poppler based, using a volume for NATS JetStream storage
podman run --rm -it -v nats:/tmp/nats -p 8080:8080 -p 4222:4222 tes:poppler-alpine

# PDFium based (external lib) without persistency
podman run -p 8080:8080 -it --rm tes:pdfium-ubuntu

# PDFfim based (LibreOffice supplied lib) without persistency
podman run -p 8080:8080 -it --rm tes:pdfiumlo-ubuntu
```

## Config

Configuration happens through environment variables only.

| Environment Variable                  | Description                                                                                                                                                                                    |
|---------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `TES_BUCKET`                          | Name of the object store bucket in NATS to use for caching. It is being created when it doesn't exist. Default: `TES_PLAINTEXTS`                                                               |
| `TES_REPLICAS`                        | Replication factor for object store bucket in external NATS cluster                                                                                                                            |
| `TES_EXPOSE_NATS`                     | Wether to allow connections to the embedded NATS server (bool)                                                                                                                                 |
| `TES_NATS_HOST`                       | Listen host/IP of embedded NATS server when `TES_EXPOSE_NATS`is `true`. Default: `localhost`                                                                                                   |
| `TES_NATS_PORT`                       | Listen Port (TCP) of embedded NATS server when `TES_EXPOSE_NATS`is `true`. Default: `4222`                                                                                                     |
| `TES_NATS_STORE_DIR`                  | Storage path for the embedded NATS server. Default: `/tmp/nats`                                                                                                                                |
| `TES_MAX_PAYLOAD`                     | Max message payload of embedded NATS server. Default: `8 MiB`                                                                                                                                  |
| `TES_NATS_URL`                        | URL of external NATS server/cluster. If this is set, no embedded NATS server is started                                                                                                        |
| `TES_FAIL_WITHOUT_JS`                 | If enabled the service exits when JetStream support of the external NATS server/cluster can not be confirmed. Default: `true`                                                                  |
| `TES_NATS_TIMEOUT`                    | Connection timeout as a `time.Duration` string. Default: `15s`                                                                                                                                 |
| `TES_NATS_CONNECT_RETRIES`            | Number of times a connection to an external NATS server/cluster and to JetStream is being tried. Default: `10`                                                                                 |
| `TES_HOST_PORT`                       | Listen address of HTTP server. Default: `:8080` (same as `0.0.0.0:8080`)                                                                                                                       |
| `TES_NO_HTTP`                         | If `true` and `TES_EXPOSE_NATS` is `true`, too, no HTTP server is started                                                                                                                      |
| `TES_REMOVE_NEWLINES`                 | If true, extracted text will be compacted by replacing newlines with whitespace (Default: `true`).                                                                                             |
| `TES_FORK_THRESHOLD`                  | Maximum content length (size in bytes) of a file that is being converted in-process rather than by a subprocess in fork-exec style. Choose a negative value to disable forking. Default: 2 MiB |
| `TES_HTTP_CLIENT_DISABLE_COMPRESSION` | Disable `Accept-Encoding: gzip` header in outgoing HTTP Requests (Default: `false`)                                                                                                            |
| `TES_TESSERACT_LANGS`                 | Set languages for Tesseract OCR as a list of 3-letter-codes, separated by `+` (Default: `eng+osd` = English + script and orientation detection)                                                |

## Security considerations

TES is not intended to be exposed to the internet.
There is no TLS support and no authentication whatsoever.
There is also no safeguard against malicious clients doing denial of service attacks by sending large files etc.

⚠️ TL;DR: Only expose this service to trusted clients in a secure environment. ⚠️

## Usage

### CLI/One-shot usage

You can supply a local file or one served via HTTP(s).

```shell
./tes /tmp/my-example.pdf
./tes https://example.com/my.pdf
```

This will output one line with JSON encoded metadata, followed by text.

At the moment there is no elaborated command line interface supporting more customization.

ℹ️ No cache is used (queried or updated) in this mode.

### Run as a service

Build and run the service, e.g. `go run -tags pdfium,nomsgpack`.
Use it as follows:

```shell
$ # POST a local file to the service
$ curl -sSi --data-binary @some-file.pdf localhost:8080
HTTP/1.1 200 OK
X-Doctype: pdf
X-Document-Author: John Doe
X-Document-Created: 2013-09-01T12:55:56+02:00
X-Document-Modified: 2013-09-01T12:55:56+02:00
X-Document-Pages: 1
X-Document-Title: Some Title
X-Document-Version: PDF-1.5
X-Parsed-By: PDFium
X-Request-Id: 3a6442af-e8bc-40b2-b0ce-84fa2b41f920
Date: Sun, 11 Aug 2024 17:43:51 GMT
Content-Length: 649
Content-Type: text/plain; charset=utf-8

Some text from some PDF file...

$ # Request some external web-hosted file
$ curl -Ssi 'localhost:8080?url=https://assets.avm.de/files/docs/fritzbox/FRITZ!Box%207690/FRITZ!Box%207690_qig_de_DE.pdf'
HTTP/1.1 200 OK
Etag: "60c3ea-61b15cff07c5e"
Http-Content-Length: 6341610
Http-Last-Modified: Mon, 17 Jun 2024 13:19:17 GMT
X-Doctype: pdf
X-Document-Created: 2024-04-11T15:06:17+02:00
X-Document-Modified: 2024-04-11T15:06:45+02:00
X-Document-Pages: 14
X-Document-Version: PDF-1.7
X-Parsed-By: PDFium
X-Request-Id: a4c2a1a1-d85e-4dfc-b122-eb6fdaafc3a3
Date: Sun, 11 Aug 2024 18:39:27 GMT
Content-Type: text/plain; charset=utf-8
Transfer-Encoding: chunked

Kurzanleitung Lieferumfang Abbildung Anzahl und Bezeichnung FON 1 Info Connect/WPS FonWLAN /DECT Power/DSL 1 FRITZ!Box 7690 1 Netzteil 1 DSL-Kabel 1 LAN-Kabel 1 TAE-Adapter ohne Abbildung 1 FRITZ! Notiz ohne Abbildung 1...

# Try an image (with tesseract installed)
$ curl  -Ssi 'localhost:8080?url=https://i0.hdslb.com/bfs/article/7353241a421b62940df9fcf579ad49ae6071cd7a.jpg'
HTTP/1.1 200 OK
Etag: 46017c6b342d49d5d28f401c09684abb
Http-Content-Length: 150406
Http-Last-Modified: Wed, 08 Sep 2021 15:51:50 GMT
X-Doctype: image/jpeg
X-Request-Id: 500dfbf5-91bf-40a5-bb33-922326249b0b
Date: Sun, 22 Sep 2024 17:29:16 GMT
Content-Length: 47
Content-Type: text/plain; charset=utf-8

Programming With bo (GOlang) And It's Benefits
```


There are a few request options you can add as query params (behind `?`/`&` in the URL):

| Param with value | Description                                                      |
|------------------|------------------------------------------------------------------|
| `noCache=true`   | Force extracting the files content, bypassing the cache          |
| `silent=true`    | Only update the cache and send the metadata, but not the content |
