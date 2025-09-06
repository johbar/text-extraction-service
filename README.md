# text-extraction-service

TES is a simple Go service for extracting and storing textual content from PDF and office documents.

## Status

This started as an exercise in using Golang and cgo.
But it is being used in production (at least for PDFs).
The use case is the fast processing of binary documents for repeated search machine indexation (see blow for details).

The RegEx-based RTF parser is rather inefficient.

The parser for XML-based office formats is not very sophisticated and might need more testing.

Apache [Tika](https://tika.apache.org/) is definitively a more versatile and mature solution to be considered.

## Features

- Support for common document formats:
  - PDF
  - RTF
  - ODT and ODP
  - DOCX and PPTX
  - legacy MS Word (.doc) files (with external helper)
- Support for three runtime-pluggable C/C++ PDF engines
    - Google Chromium's [PDFium](https://pdfium.googlesource.com/pdfium/)
    - Free Desktops [Poppler](https://poppler.freedesktop.org/)
    - Artifex' [MuPDF](https://mupdf.com/)
- Dehyphenation of extracted text, specifically for German (only an issue with PDFs)
- Extraction of document metadata (title, author, creation date etc)
- Store extracted text and metadata in NATS for faster retrieval
- NATS can be embedded or run externally (e.g. as a cluster)
- Support for NATS microservice interface
- (Experimental) Optical character recognition by [Tesseract OCR](https://github.com/tesseract-ocr/) (useful for images containing text and scanned PDFs)

## Unsupported

- Processing local files with the `file:` transport
- Processing password protected files
- Processing files from web servers that require authentication of any kind (cookie, header, referral, user agent etc)
- a lot of file formats, e.g. ppt, markdown, ods/xlsx, html

## License

This service inherits the Open Source license of the PDF lib used at runtime:

- PDFium: [Apache-2](https://pdfium.googlesource.com/pdfium/+/master/LICENSE)
- Poppler: GPL-2.0
- MuPDF: AGPL-3.0 (commercial license available)

This approach is unusual and probably problematic.
There is an unresolved debate about the consequences of linking against GPL libraries concerning the licensing obligations of a software.
With the current `purego`-based implementation there is not even a compile-time dependency on Poppler or MuPDF.
But these libraries can still be loaded at runtime and will run in the same address space, just like any other dynamically linked library.

I'm not an OSS license expert but considering these issues putting TES under GPL by default seems the safest approach to be compliant, even if, with Apache-2-licensed PDFium being the default implementation, there is no actual integration with GPL libs by default.

Nevertheless, if you intend to fork this project and remove the GPL-related code in favor of PDFium only, feel free to put it under Apache-2 license.

## Design Goals (and some words on the development of TES)

Coming from a Java stack I wanted to build a solution for PDF to text conversion that offered better quality and performance than Apache PDFBox and Tika.
Poppler and MuPDF were well-established PDF libs with a C interface I examined and included in this service at first.
Later I discovered PDFium as an additional FOSS solution.

Initially I used `cgo` to integrate these libs with Go (using open source wrappers like [go-pdfium](https://github.com/klippa-app/go-pdfium) and [go-fitz](https://github.com/gen2brain/go-fitz) or even writing my own for Poppler).
Build tags were used to decide which lib to link against.
Later I facilitated [purego](https://github.com/ebitengine/purego) to get rid of it all.
No more `cgo`, C header files, `pkg-config` or build tags!
There are still a lot of runtime dependencies, but the build system can be as simple as `go build`, including cross compiling.

Using C/C++ PDF libs to do the heavy lifting of text extraction is one part of the solution.
The other is the integration of an optional cache and the algorithm used when serving extraction requests:

1. Ask the cache for metadata about the requested document.
2. If metadata is available, do an "optional" HTTP request using `If-Non-Match` and/or `If-Modified-Since` headers with entity tags/timestamps. If not, just fetch the doc.
3. If the webserver sends the document (instead of status `304 Not Modified`) do text extraction. Otherwise read the text from cache.
4. Send the metadata as HTTP headers and the text as plain UTF-8 body. No JSON serialization needed.
5. If indicated, do the cache update after serving the request, so the client can proceed with what ever they wanted to do with the text.

Additional design considerations and assumptions:

- Do everything in-memory and in-process, whenever you can. No disk I/O for small files, no invocation of external programs in most cases (exceptions being `wvWare`/`antword`/`catdoc` for DOCs and `tesseract` for OCR).
- The web service client does not care that much about, say, the PDF itself, but rather the textual content and some metadata.
  They don't want to download it to post it to TES, but they know the URL, so that's all TES needs to do the job.
- The client does not care that much about the PDFs layout as they do about its textual content.
  So text returned by TES should only be semantically correct concerning the order of words on pages etc. but not accurate in presentation.
  ➡️> Join words that have been split up by hyphens on line endings, remove newlines in order to save bandwidth etc.

## Quick start

On the [releases page](https://github.com/johbar/text-extraction-service/releases/) you find binaries that have `libpdfium.so`/`libpdfium.dylib`/`pdfium.dll` embedded.
See latest releases below.

| OS      | Variant | Architecture | PDFium embedded                                                                                                            | Solo                                                                                                                                     |
|---------|---------|--------------|----------------------------------------------------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------|
| Linux   | glibc   | amd64        | [tes-linux-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-linux-amd64.gz)           | [tes-linux-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-linux-amd64.gz)                  |
| Linux   | glibc   | arm64        | [tes-linux-arm64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-linux-arm64.gz)           | [tes-pdfium-linux-arm64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-linux-arm64.gz)           |
| Linux   | musl    | amd64        | [tes-musl-linux-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-musl-linux-amd64.gz) | [tes-pdfium-musl-linux-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-musl-linux-amd64.gz) |
| Linux   | musl    | arm64        | [tes-musl-linux-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-musl-linux-arm64.gz) | [tes-pdfium-musl-linux-arm64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-musl-linux-arm64.gz) |
| MacOs   |         | arm64        | [tes-mac-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-mac-amd64.gz)               | [tes-pdfium-mac-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-mac-amd64.gz)               |
| MacOs   |         | amd64        | [tes-mac-arm64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-mac-arm64.gz)               | [tes-pdfium-mac-arm64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-mac-arm64.gz)               |
| Windows |         | amd64        | [tes-windows-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-windows-amd64.gz)       | [tes-pdfium-windows-amd64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-windows-amd64.exe.gz)   |
| Windows |         | arm64        | [tes-windows-arm64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-windows-arm64.exe.gz)   | [tes-pdfium-windows-arm64](https://github.com/johbar/text-extraction-service/releases/latest/download/tes-pdfium-windows-arm64.exe.gz)   |

On *nix systems you need to `chmod +x` the binary after `gunzip`ing it.

## Dev Setup - Building TES

Building only requires a recent Go SDK (v1.24.1) thanks to `purego`.
But testing and running TES requires additional shared libs.
Depending on the PDF engine you choose (see below for comparison) you need it installed in your dev/build environment.

The `docparser` package (for legacy MS Word files) can work with either of the CLI tools `wvWare`, `antiword` or `catdoc`.

All instructions supplied here suppose a Linux environment.

### PDFium

PDFium is the default implementation and works without any configuration as long as the lib is present in a standard path.

If you have LibreOffice installed you don't necessarily need to download PDFium.
TES will use `/usr/lib/libreoffice/program/libpdfiumlo.so` which is compatible with `libpdfium.so`, if available.

Otherwise or if you prefer a current version of the upstream lib:

- Download the correct PDFium binary for your platform from [bblanchon/pdfium-binaries](https://github.com/bblanchon/pdfium-binaries) or compile the lib yourself.
  You can also use the [downloader script](./pkg/pdflibwrappers/pdfium_purego/download-pdfium.sh) to do so.
- Put `libpdfium.so` in `/usr/local/lib/` (my recommendation).
- Set the config env variable in your shell via `export TES_PDF_LIB_PATH=/path/to/libpdfium.so` if you put elsewhere.

### Poppler (`poppler-glib`)

- Install dependencies on Debian based systems via `apt-get install libpoppler-glib8`.
- `export TES_PDF_LIB_NAME=poppler` in your shell.
- If TES cannot load the lib, file a bug report please and try setting `export TES_PDF_LIB_PATH=/path/to/libpoppler-glib.so` in your shell.

### MuPDF

- Have look at [this Containerfile](Containerfile.mupdf-ubuntu) to get an idea of how to build MuPDF as a shared lib for Debian or Ubuntu.
  You can also build that image, run it and then copy the lib to your host system via `podman cp`.
- Put `libmupdf.so` in `/usr/local/lib/`.
- Set the config env variable in your shell via `export TES_PDF_LIB_PATH=/path/to/libmupdf.so` if you put elsewhere.
- `export TES_PDF_LIB_NAME=mupdf` in your shell.
- Set `MUPDF_VERSION` according to the lib version.

## Build locally

```sh
go build -o tes
```

## Embed NATS

If you want to run NATS embedded in TES with zero config use the build tag `embed_nats`.

```sh
go build -tags embed_nats -o tes
```

## OCR (experimental)

If you want to process image files or scanned PDFs TES got you covered.
All you need to do is:

1. Install Tesseract, e.g. on Debian/Ubuntu run `apt install tesseract-ocr`
2. Install any language model file you need. English is included by default.
    1. Run `apt install tesseract-ocr-script-latn` for a model that supports multiple languages with latin script (rather large and slow).
    2. Or run `apt install tesseract-ocr-deu` for the german model (smaller but specific, hence faster).
3. Configure TES to pass on your language preference to Tesseract by setting the environment variable, e.g. `TES_TESSERACT_LANGS=Latin+osd` when running TES.
4. Optionally set `TESS_PREFIX` if you have installed the language models in a custom path.

If there is no text found on a PDF page and Tesseract is available TES will look for images on that page.
It will then extract these images in-memory and pipe them to the Tesseract CLI.
The output is then streamed back to TES or rather the client (after dehyphenation and compaction).

This means: If there is a mixture of text and images on a page, no OCR at all is being performed.

NOTE: OCR is an expensive process and can take a lot of time and resources.
It is not fully accurate.

NOTE: This feature is not well tested and thus considered experimental.

##  PDFium, MuPDF or Poppler?

Concerning the quality of text extracted by theses libs in my experience *Poppler* and *PDFium* are a bit better than *MuPDF*.
But complicated as the Portable Document Format is there are a lot of edge cases one lib handles better than the other–and some where neither can do right.

Regarding speed with ordinary (rather small) files *PDFium* and *MuPDF* are mostly astride.

Some other aspects:

|                                             | PDFium                   | Poppler              | MuPDF                        |
|---------------------------------------------|--------------------------|----------------------|------------------------------|
| License                                     | ✅ permissive             | ⚠️ Copyleft          | ⚠️ Copyleft or payed license  |
| Performance with small files                | ✅ good                   | ❌ bad              | ✅ good                      |
| Performance with large files                | ✅ good                   | 🚀 best             | ❌ bad                       |
| Memory consumption                          | ❌ high with large files¹ | ✅ consistently low | ❌ high with large files     |
| Available from Linux sources(deb, rpm, apk) | ❌ no, but...¹            | ✅ yes              | ✔️ partially²                 |
| Multi-threaded                              | ❌ no³                    | ✅ yes              | ✅ yes                       |

¹ At runtime you can use the LibreOffice build of *PDFium*, `libpdfiumlo.so` from the Debian/Ubuntu package `libreoffice-core-nogui`.
Using this lib instead of [bblanchon/pdfium-binaries](https://github.com/bblanchon/pdfium-binaries) performance drops a bit (maybe 10%), but in turn memory consumption with large files decreases a lot.
See [Containerfile](Containerfile.pdfiumlo-ubuntu) on how to use this shared lib.

² Debian and Ubuntu only ship static libs of MuPDF, Alpine and Arch have shared libs as well. 

³ *PDFium* is not thread safe.
For that reason TES uses a lock to protect the lib instance against concurrent access and a multi-processing approach (forking a new TES process, connected with pipes, if PDFium is busy)

## Build container images

This repo includes a bunch of Containerfiles for building images in a multi-stage style for various setups (PDF lib, Tesseract).

I use Podman but everything should work with Docker as well.

Ubuntu refers to Ubuntu 22.04 (*Jammy*).

```sh
# optional: use a volume to speed up subsequent builds—remove the need to re-download and re-compile all dependencies
mkdir --mode 777 --parents /tmp/cache

# All-in-one on Alpine:
podman build --pull . -f Containerfile.alpine -t tes:alpine --volume /tmp/cache:/tmp

# MuPDF on Alpine:
podman build --pull . -f Containerfile.mupdf-alpine -t tes:mupdf-alpine --volume /tmp/cache:/tmp

# MuPDF on Ubuntu:
podman build --pull . -f Containerfile.mupdf-ubuntu -t tes:mupdf-ubuntu --volume /tmp/cache:/tmp

# Poppler on Alpine:
podman build --pull . -f Containerfile.poppler-alpine -t tes:poppler-alpine --volume /tmp/cache:/tmp

# Poppler on Ubuntu:
podman build --pull . -f Containerfile.poppler-ubuntu -t tes:poppler-ubuntu --volume /tmp/cache:/tmp

# PDFium on Ubuntu:
podman build --pull . -f Containerfile.pdfium-ubuntu -t tes:pdfium-ubuntu --volume /tmp/cache:/tmp

# PDFium from Libreoffice on Ubuntu
podman build --pull . -f Containerfile.pdfiumlo-ubuntu -t tes:pdfiumlo-ubuntu --volume /tmp/cache:/tmp

# PDFium (LO) + Tesseract (Latin/multi-lang) on Ubuntu
podman build --pull . -f Containerfile.pdfiumlo-tesseract-latin-ubuntu -t tes:pdfiumlo-ocr-ubuntu --volume /tmp/cache:/tmp
```

## Run TES in a container (examples)

```sh
# MuPDF based, using a volume for NATS JetStream storage
podman run --rm -it -v nats:/tmp/nats -p 8080:8080 -p 4222:4222 tes:mupdf-alpine

# Poppler based, using a volume for NATS JetStream storage
podman run --rm -it -v nats:/tmp/nats -p 8080:8080 -p 4222:4222 tes:poppler-alpine

# PDFium based (external lib) without persistency
podman run -p 8080:8080 -it --rm tes:pdfium-ubuntu

# PDFfim based (LibreOffice supplied lib) without persistency
podman run -p 8080:8080 -it --rm tes:pdfiumlo-ubuntu

# PDFium based with Tesseract installed
podman run --rm -it -v nats:/tmp/nats -p 8080:8080 -p 4222:4222 tes:pdfiumlo-ocr-ubuntu
```

## Config

Configuration happens through environment variables only.

| Environment Variable                  | Description                                                                                                                                                                                    |
|---------------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `TES_BUCKET`                          | Name of the object store bucket in NATS to use for caching. It is being created when it doesn't exist. Default: `TES_PLAINTEXTS`                                                               |
| `TES_REPLICAS`                        | Replication factor for object store bucket in external NATS cluster                                                                                                                            |
| `TES_EXPOSE_NATS`                     | Wether to allow connections to the embedded NATS server (bool). Default: `false`                                                                                                               |
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
| `TES_PDF_LIB_NAME`                    | Name of the PDF implementation to load; options: `pdfium` (default), `poppler`, `mupdf`                                                                                                        |
| `TES_PDF_LIB_PATH`                    | Path or basename of the shared lib (`.so`, `.dylib`, `.dll`); if empty some default names and paths are tried                                                                                  |
| `TES_REMOVE_NEWLINES`                 | If true, extracted text will be compacted by replacing newlines with whitespace. Default: `true`                                                                                               |
| `TES_FORK_THRESHOLD`                  | Maximum content length (size in bytes) of a file that is being converted in-process rather than by a subprocess in fork-exec style. Choose a negative value to disable forking. Default: 2 MiB |
| `TES_MAX_IN_MEMORY`                   | Maximum size a file may have to be processed in-memory. Is a file larger, it will be downloaded to `$TMP`. Default: `2MiB`                                                                     |
| `TES_MAX_FILE_SIZE`                   | Maximum size a file may have to be processed. Larger files will be discarded. Default `300MiB`                                                                                                 |
| `TES_HTTP_CLIENT_DISABLE_COMPRESSION` | Disable `Accept-Encoding: gzip` header in outgoing HTTP Requests. Default: `false`                                                                                                             |
| `TES_TESSERACT_LANGS`                 | Set languages for Tesseract OCR as a list of 3-letter codes or script identifiers, separated by `+`. Default: `Latin` = all languages with latin script                                                              |
| `TES_LOG_LEVEL`                       | Sets the log level. Options (case-insensitive): `info` (default), `debug`, `warn`, `error`                                                                                                     |
| `TES_DEBUG`                           | Adds source info to each log line. Default: `false`                                                                                                                                            |

## Security considerations

TES is not intended to be exposed to the internet.
There is no TLS support and no authentication whatsoever.
There is also no safeguard against malicious clients doing denial of service attacks by sending large or prepared files etc.

⚠️ TL;DR: Only expose this service to trusted clients and documents in a secure environment. ⚠️

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

Build and run the service, e.g. `go run` or use a binary supplied at the [releases page](https://github.com/johbar/text-extraction-service/releases/).
Use it as follows:

```shell
$ # POST a local file to TES
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
X-Document-Creator: Adobe InDesign 19.3 (Windows)
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

## NATS Microservice interface (experimental)

If you are a friend of NATS.io you can interact with TES via NATS request/reply.
At the moment there are two endpoints/subjects available TES instances connected to NATS (embedded or external) subscribe to:

- `update-cache` with an URL as payload. TES will validate or update the cache entry for the specified URL and reply with a simple `done`
- `extract-remote` with a simple JSON Payload representing the query parameters of an equivalent HTTP request.

This feature is considered experimental insofar it lacks customizability and might be subject to change.

Examples using the NATS CLI:

```shell
# Start TES with TES_EXPOSE_NATS=true, embedded NATS will listen on standard port:
$ TES_EXPOSE_NATS=true ./tes &
# Call NATS CLI:
$ nats request extract-remote '{"url": "https://assets.avm.de/files/docs/fritzbox/FRITZ!Box%207690/FRITZ!Box%207690_qig_de_DE.pdf", "noCache": true}'
23:48:23 Sending request on "extract-remote"
23:48:24 Received with rtt 1.495585644s
23:48:24 X-Doctype: pdf
23:48:24 X-Document-Creator: Adobe InDesign 19.3 (Windows)
23:48:24 X-Document-Modified: 2024-04-11T15:06:45+02:00
23:48:24 X-Document-Producer: Adobe PDF Library 17.0
23:48:24 X-Parsed-By: PDFium
23:48:24 Etag: "60c3ea-61b15cff07c5e"
23:48:24 Http-Content-Length: 6341610
23:48:24 X-Document-Pages: 14
23:48:24 X-Document-Version: PDF-1.7
23:48:24 Http-Last-Modified: Mon, 17 Jun 2024 13:19:17 GMT
23:48:24 X-Document-Created: 2024-04-11T15:06:17+02:00

Kurzanleitung Lieferumfang Abbildung Anzahl und Bezeichnung FON 1 Info...
```
