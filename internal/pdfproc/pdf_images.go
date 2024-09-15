// Package pdfproc implements a limited set of operations to process PDFs
package pdfproc

import (
	"io"
	"strconv"
	"time"

	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
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
}

// GetImages returns a slice of all images present on the page with number pageNum.
// func GetImages(pdf io.ReadSeeker, pageNum int) ([]model.Image, error) {
// 	page := []string{strconv.Itoa(pageNum+1)}
// 	mapSlice, err := pdfcpuapi.Images(pdf, page, pdfConf)
// 	if err != nil {
// 		return nil, err
// 	}
// 	var result []model.Image
// 	if len(mapSlice) == 0  {
// 		return nil, errors.New("no images found")
// 	}
// 	log.Printf("mapslice: %v", mapSlice)
// 	pageImages := mapSlice[0]
// 	for k, img := range pageImages {
// 		log.Printf("%v: %v", k, img)
// 		result = append(result, img)
// 	}
// 	return result, err
// }

func ExtractImages(rs io.ReadSeeker, pageIndex int, readFunc func(model.Image)) {
	pageStr := []string{strconv.Itoa(pageIndex + 1)}
	// var images []model.Image
	pdfcpuapi.ExtractImages(rs, pageStr, func(img model.Image, singleImgPerPage bool, maxPageDigits int) error {
		readFunc(img)
		return nil
	}, pdfConf)

}

// func digestImage(images chan model.Image) func(model.Image, bool, int) error {
// 	return func(img model.Image, singleImgPerPage bool, maxPageDigits int) error {
// 		images <- img
// 		return nil
// 	}
// }

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
