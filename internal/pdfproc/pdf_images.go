// Package pdfproc implements a limited set of operations to process PDFs
package pdfproc

import (
	"bytes"
	"os"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type PdfMetaData struct {
	Author, Title, Subject string
	Created, Modified      time.Time
	PageCount              int
}

var pdfConf *model.Configuration

func init() {
	pdfConf = model.NewDefaultConfiguration()
	pdfConf.ValidateLinks = false
	pdfConf.Offline = true
	pdfConf.Cmd = model.EXTRACTIMAGES
}

// ParseForImageExtraction parses a PDF file in-memory for extracting images
func ParseForImageExtraction(pdfData []byte) (*model.Context, error) {
	var ctx *model.Context
	rs := bytes.NewReader(pdfData)
	ctx, err := api.ReadValidateAndOptimize(rs, pdfConf)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

// ParsePathForImageExtraction parses a PDF file on-disk for extracting images
func ParsePathForImageExtraction(path string) (*model.Context, error) {
	var ctx *model.Context
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	ctx, err = api.ReadValidateAndOptimize(f, pdfConf)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func GetImages(ctx *model.Context, page int) ([]model.Image, error) {
	// pdfcpu page numbers start at 1, ours at 0
	images, err := pdfcpu.ExtractPageImages(ctx, page+1, false)
	if err != nil {
		return nil, err
	}

	var imgSlice ([]model.Image)
	for _, img := range images {
		imgSlice = append(imgSlice, img)
	}
	return imgSlice, nil
}
