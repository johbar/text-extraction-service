package pdftextextractor

import (
	"compress/zlib"
	"errors"
	"fmt"
	"io"

	"github.com/pdfcpu/pdfcpu/pkg/filter"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PageContentInto appends the content in PDF syntax for page dict d to buf.
// This enables (external) memory pooling.
func pageContentInto(xRefTable *model.XRefTable, d types.Dict, pageNr int, buf io.Writer) error {
	o, _ := d.Find("Contents")
	if o == nil {
		return model.ErrNoContent
	}

	o, err := xRefTable.Dereference(o)
	if err != nil || o == nil {
		return err
	}

	n := 0
	switch o := o.(type) {

	case types.StreamDict:
		// no further processing.
		if err := decodeContentStream(xRefTable, &o, pageNr); err != nil {
			return err
		}
		n, _ = buf.Write(o.Content)

	case types.Array:
		// process array of content stream dicts.
		for _, o := range o {
			if o == nil {
				continue
			}
			o, _, err := xRefTable.DereferenceStreamDict(o)
			if err != nil {
				return fmt.Errorf("page %d content decode: %v", pageNr, err)
			}
			if o == nil {
				continue
			}
			if err := decodeContentStream(xRefTable, o, pageNr); err != nil {
				return err
			}
			m, _ := buf.Write(o.Content)
			n += m
		}

	default:
		return fmt.Errorf("pdfcpu: page content must be stream dict or array")
	}

	if n == 0 {
		return model.ErrNoContent
	}

	return nil
}

// modified version of pdfcpu@v0.16.1/pkg/pdfcpu/model/xreftable.go
//	Copyright 2021 The pdfcpu Authors.
// Licensed under the Apache License, Version 2.0
func decodeContentStream(xRefTable *model.XRefTable, sd *types.StreamDict, pageNr int) error {
	err := sd.Decode()
	if errors.Is(err, filter.ErrUnsupportedFilter) {
		return fmt.Errorf("page %d content decode: %w", pageNr, err)
	}
	if err != nil {
		if xRefTable.ValidationMode == model.ValidationStrict {
			return fmt.Errorf("page %d content decode: %w", pageNr, err)
		}
		if !filter.IsCorruptFlateInput(err) && !errors.Is(err, zlib.ErrChecksum) {
			return fmt.Errorf("page %d content decode: %w", pageNr, err)
		}
	}
	return nil
}
