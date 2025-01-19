package main

import (
	"bytes"
	"io"

	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

type ImageDoc struct {
	data     *[]byte
	mimetype string
}

func NewDocFromImage(data []byte, mimetype string) *ImageDoc {
	return &ImageDoc{data: &data, mimetype: mimetype}
}

func (d *ImageDoc) StreamText(w io.Writer) error {
	return tesswrap.ImageReaderToTextWriter(bytes.NewReader(*d.data), w)
}

func (d *ImageDoc) Close() {
	// no op
}

func (d *ImageDoc) Pages() int {
	return -1
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
	meta["x-doctype"] = d.mimetype
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
