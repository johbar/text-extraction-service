package docparser

import (
	"fmt"
	"io"
	"sync"

	"github.com/richardlehane/mscfb"
)

// Open opens the .doc file at path and returns a Doc ready for text extraction
// and metadata access. The CFB directory is walked exactly once: all relevant
// stream bytes are collected, the WordDocument entry is kept as an io.ReaderAt
// for lazy per-piece reading, and the two OLE property streams are stashed for
// lazy metadata parsing.

// Metadata returns the document's OLE property set metadata.
// The property streams are parsed on the first call and the result is cached;
// subsequent calls return the same pointer without re-parsing.

// docStreams holds everything collected from a single CFB directory pass.
type docStreams struct {
	// Text extraction in Word files
	wordDocument io.ReaderAt // mscfb entry kept open for per-piece ReadAt
	metaErr      error
	metaVal      *Metadata
	table        []byte // whichever of 0Table/1Table the FIB selects
	// streams only present in PowerPoint files
	pptDoc      []byte
	currentUser []byte

	// Metadata — raw bytes stashed at open time, parsed lazily on first request.
	siRaw       []byte // \x05SummaryInformation stream, nil if absent
	dsiRaw      []byte // \x05DocumentSummaryInformation stream, nil if absent
	wordDocSize int64
	// Lazy metadata cache.
	metaOnce sync.Once
}

// metadata parses the OLE property streams on the first call and caches the
// result. Subsequent calls return the cached value without re-parsing.
func (d *docStreams) metadata() (*Metadata, error) {
	d.metaOnce.Do(func() {
		m := &Metadata{}
		if d.siRaw != nil {
			if err := parseSummaryInformation(d.siRaw, m); err != nil {
				d.metaErr = fmt.Errorf("SummaryInformation: %w", err)
				return
			}
		}
		if d.dsiRaw != nil {
			if err := parseDocumentSummaryInformation(d.dsiRaw, m); err != nil {
				d.metaErr = fmt.Errorf("DocumentSummaryInformation: %w", err)
				return
			}
		}
		d.metaVal = m
	})
	return d.metaVal, d.metaErr
}

/*
openDocStreams opens the CFB from a reader and populates a docStreams struct in a single directory pass.

Two stream types are collected for Word and PPT files:

	\x05SummaryInformation     — read fully; stashed for lazy metadata parse
	\x05DocumentSummaryInformation — same

mscfb strips the leading 0x05 from non-printable stream names, so we match on
entry.Initial == 0x0005 combined with the bare name.

The other streams are either exclusive for ppt or doc.
For doc:

	WordDocument               — kept as io.ReaderAt (not fully buffered)
	0Table / 1Table            — read fully; FIB selects which one to keep

For ppt:

	PowerPoint Document
	Current User
*/
func openDocStreams(f io.ReaderAt) (*docStreams, error) {
	d := &docStreams{}
	cfb, err := mscfb.New(f)
	if err != nil {
		return nil, err
	}
	var wordDocEntry *mscfb.File
	var tbl0, tbl1 []byte
	var fibRaw []byte

	for entry, cerr := cfb.Next(); cerr == nil; entry, cerr = cfb.Next() {
		// Property streams are identified by Initial==0x05 (mscfb strips it
		// from Name) plus the bare name.
		if entry.Initial == 0x0005 {
			switch entry.Name {
			case "SummaryInformation":
				d.siRaw, err = io.ReadAll(cfb)
				if err != nil {
					return nil, fmt.Errorf("read SummaryInformation: %w", err)
				}
			case "DocumentSummaryInformation":
				d.dsiRaw, err = io.ReadAll(cfb)
				if err != nil {
					return nil, fmt.Errorf("read DocumentSummaryInformation: %w", err)
				}
			}
			continue
		}

		switch entry.Name {
		case "WordDocument":
			wordDocEntry = entry
			d.wordDocSize = entry.Size
			// Read only the FIB — enough to locate the Table stream and Clx.
			// 154 pre-blob bytes + pair 33 at blob offset 264 + 8 = 426 min;
			// 512 is a safe round number and always present in valid files.
			fibRaw = make([]byte, min64(512, entry.Size))
			if _, err = entry.ReadAt(fibRaw, 0); err != nil && err != io.EOF {
				return nil, fmt.Errorf("read FIB: %w", err)
			}
		case "0Table":
			tbl0, err = io.ReadAll(cfb)
			if err != nil {
				return nil, fmt.Errorf("read 0Table: %w", err)
			}
		case "1Table":
			tbl1, err = io.ReadAll(cfb)
			if err != nil {
				return nil, fmt.Errorf("read 1Table: %w", err)
			}
		case "PowerPoint Document":
			d.pptDoc, err = io.ReadAll(cfb)
			if err != nil {
				return nil, fmt.Errorf("read PowerPoint Document: %w", err)
			}
		case "Current User":
			d.currentUser, err = io.ReadAll(cfb)
			if err != nil {
				return nil, fmt.Errorf("read Current User: %w", err)
			}
		}
	}

	if wordDocEntry == nil && d.pptDoc == nil {
		return nil, fmt.Errorf("Neither WordDocument nor PowerPoint Document stream found")
	}

	if wordDocEntry != nil {
		if len(fibRaw) < 32 {
			return nil, fmt.Errorf("WordDocument stream too short for FibBase")
		}

		// fWhichTblStm: bit 9 of the FibBase flags word at offset 10.
		fWhichTblStm := (le16(fibRaw, 10) >> 9) & 1
		switch {
		case fWhichTblStm == 0 && tbl0 != nil:
			d.table = tbl0
		case fWhichTblStm == 0 && tbl1 != nil:
			d.table = tbl1
		case fWhichTblStm == 1 && tbl1 != nil:
			d.table = tbl1
		case fWhichTblStm == 1 && tbl0 != nil:
			d.table = tbl0
		default:
			return nil, fmt.Errorf("no Table stream found")
		}
		d.wordDocument = wordDocEntry
	}

	return d, nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
