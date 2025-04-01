package main

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

type ImageDoc struct {
	data *[]byte
	typ  string
	path string
}

func NewDocFromImage(data []byte, ext string) *ImageDoc {
	if data == nil {
		return nil
	}
	return &ImageDoc{data: &data, typ: strings.TrimPrefix(ext, ".")}
}

func OpenImage(path, ext string) *ImageDoc {
	return &ImageDoc{path: path, typ: strings.TrimPrefix(ext, ".")}
}

func (d *ImageDoc) StreamText(w io.Writer) error {
	if d.data != nil {
		return tesswrap.ImageReaderToWriter(bytes.NewReader(*d.data), w)
	}
	if len(d.path) > 0 {
		return tesswrap.ImageToWriter(d.path, w)
	}
	return errors.New("image has neither bytes nor path")
}

func (d *ImageDoc) Close() {
	// no op
}

func (d *ImageDoc) Pages() int {
	return -1
}

func (d *ImageDoc) Path() string {
	return d.path
}

func (d *ImageDoc) Data() *[]byte {
	return d.data
}

func (d *ImageDoc) Text(i int) (string, bool) {
	if i != 1 {
		return "", false
	}
	text, err := tesswrap.ImageBytesToText(*d.data)
	if err != nil {
		logger.Error("Tesseract failed")
	}
	// an image has no image
	return text, false
}

func (d *ImageDoc) MetadataMap() map[string]string {
	meta := make(map[string]string)
	meta["x-doctype"] = d.typ
	// this isn't really useful and may even be expensive in terms of cpu/memory and new deps
	// so omitting it for now...

	// img, typ, err := image.Decode(bytes.NewReader(d.data))
	// if err != nil {
	// 	return
	// }

	// p := img.Bounds().Size()
	// meta["x-image-dimensions"] = fmt.Sprintf("%dx%d", p.X, p.Y)
	return meta
}
