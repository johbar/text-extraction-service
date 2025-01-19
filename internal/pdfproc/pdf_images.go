// Package pdfproc implements a limited set of operations to process PDFs
package pdfproc

import (
	"bytes"
	"io"
	"strconv"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
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

// Parses a PDF file in-memory for extracting images
func ParseForImageExtraction(pdfData []byte) (*model.Context, error) {
	var ctx *model.Context
	rs := bytes.NewReader(pdfData)
	ctx, err := api.ReadValidateAndOptimize(rs, pdfConf)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func GetImages(ctx *model.Context, page int) ([]model.Image, error) {
	images, err := pdfcpu.ExtractPageImages(ctx, page, false)
	println("len: ", len(images))
	if err != nil {
		return nil, err
	}

	var imgSlice ([]model.Image)
	for _, img := range images {
		imgSlice = append(imgSlice, img)
	}
	return imgSlice, nil
}

// ProcessImages applies readFunc to every image found on the page with the specified zero-based page number
func ProcessImages(rs io.ReadSeeker, pageIndex int, readFunc func(model.Image)) {
	pageStr := []string{strconv.Itoa(pageIndex + 1)}
	pdfcpuapi.ExtractImages(rs, pageStr, func(img model.Image, singleImgPerPage bool, maxPageDigits int) error {
		readFunc(img)
		return nil
	}, pdfConf)

}

// GetPdfInfos returns a PDF file's Metadata
func GetPdfInfos(rs io.ReadSeeker) (PdfMetaData, error) {
	info, err := pdfcpuapi.PDFInfo(rs, "", nil, nil)
	if err != nil {
		return PdfMetaData{}, err
	}
	meta := PdfMetaData{Author: info.Author, Title: info.Title, Subject: info.Subject, PageCount: info.PageCount}
	if mod, ok := types.DateTime(info.ModificationDate, true); ok {
		meta.Modified = mod
	}
	if created, ok := types.DateTime(info.CreationDate, true); ok {
		meta.Created = created
	}
	return meta, nil
}
