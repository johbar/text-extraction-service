package docparser

import (
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf16"
)

// WriteText extracts plain text from the .doc file at path and writes it to w.
//
// Text is decoded and written one piece (Pcd entry) at a time. The only data
// held in memory at once are the Table stream (structure, typically a few KB)
// and a single piece buffer. The WordDocument stream is read via random-access
// so its text content is never fully buffered.
//
// Only the Main Document story is written; headers, footers, footnotes and
// text boxes are excluded.
//
// For callers that also need metadata, use Open so the CFB directory is only
// walked once:
//
//	doc, err := doc2text.Open(path)
//	doc.WriteText(w)
//	doc.Metadata()

// writeText does the actual work once streams are open.
func writeText(d *docStreams, w io.Writer) error {
	wd := d.wordDocument

	// ── 1. Read the FIB header fields we need ────────────────────────────────
	//
	// We need:
	//   fEncrypted    — bit 8 of FibBase flags word at offset 10
	//   ccpText       — first LONG of fibRgLw (character count of main story)
	//   fcClx/lcbClx  — pair 33 of fibRgFcLcbBlob (offset/size of Clx)
	//
	// We read the minimum contiguous region: from byte 0 to byte 426
	// (154 fixed bytes before the blob + 33*8+8 = 272 bytes into the blob).
	// Round up to 512 for simplicity; the FIB is always at least that large.
	const fibReadSize = 512
	fibBuf := make([]byte, min64(fibReadSize, d.wordDocSize))
	if _, err := wd.ReadAt(fibBuf, 0); err != nil && err != io.EOF {
		return fmt.Errorf("read FIB: %w", err)
	}

	if len(fibBuf) < 32 {
		return fmt.Errorf("WordDocument stream too short for FibBase")
	}
	if wIdent := le16(fibBuf, 0); wIdent != 0xA5EC {
		return fmt.Errorf("not a Word Binary file (wIdent=0x%04X)", wIdent)
	}
	if (le16(fibBuf, 10)>>8)&1 != 0 {
		return fmt.Errorf("document is encrypted; cannot extract text")
	}

	// Navigate past FibBase → fibRgW → fibRgLw → cbRgFcLcb to reach the blob.
	off := 32
	csw := int(le16(fibBuf, off))
	off += 2 + csw*2
	ccpTextOff := off + 2
	cslw := int(le16(fibBuf, off))
	off += 2 + cslw*4
	cbRgFcLcb := int(le16(fibBuf, off))
	off += 2 // off is now at fibRgFcLcbBlob[0]

	const fcClxInBlob = 264 // pair 33 × 8 bytes
	if cbRgFcLcb*8 < fcClxInBlob+8 {
		return fmt.Errorf("FibRgFcLcb too small to contain fcClx (cbRgFcLcb=%d)", cbRgFcLcb)
	}
	if len(fibBuf) < off+fcClxInBlob+8 {
		return fmt.Errorf("FIB buffer too short to reach fcClx")
	}
	fcClx := le32(fibBuf, off+fcClxInBlob)
	lcbClx := le32(fibBuf, off+fcClxInBlob+4)

	var ccpText uint32
	if len(fibBuf) >= ccpTextOff+4 {
		ccpText = le32(fibBuf, ccpTextOff)
	}

	// ── 2. Parse the Clx → PlcPcd from the Table stream ──────────────────────
	clxEnd := fcClx + lcbClx
	if uint32(len(d.table)) < clxEnd {
		return fmt.Errorf("Table stream too short for Clx (need %d, have %d)",
			clxEnd, len(d.table))
	}
	pieces, err := parsePlcPcd(d.table[fcClx:clxEnd])
	if err != nil {
		return fmt.Errorf("parsePlcPcd: %w", err)
	}

	// ── 3. Stream each piece to w ─────────────────────────────────────────────
	for _, p := range pieces {
		cpStart := p.cpStart
		cpEnd := p.cpEnd

		if ccpText > 0 {
			if cpStart >= ccpText {
				break
			}
			if cpEnd > ccpText {
				cpEnd = ccpText
			}
		}
		nChars := cpEnd - cpStart
		if nChars == 0 {
			continue
		}

		if err := writePiece(wd, w, p.fc, p.compressed, nChars); err != nil {
			// Soft error: skip a corrupt/truncated piece rather than aborting.
			// Returning the error here would be equally reasonable; adjust to
			// taste depending on how strict you want the extractor to be.
			continue
		}
	}

	return nil
}

// ── Piece table ───────────────────────────────────────────────────────────────

type piece struct {
	cpStart    uint32
	cpEnd      uint32
	fc         uint32 // byte offset in WordDocument stream
	compressed bool   // true → 1-byte Windows-1252; false → 2-byte UTF-16LE
}

// parsePlcPcd parses a Clx blob and returns an ordered slice of pieces.
// See doc2text.go for full format documentation.
func parsePlcPcd(clx []byte) ([]piece, error) {
	off := 0
	for off < len(clx) && clx[off] == 0x01 {
		off++
		if off+2 > len(clx) {
			return nil, fmt.Errorf("truncated Prc header at byte %d", off)
		}
		off += 2 + int(binary.LittleEndian.Uint16(clx[off:]))
	}

	if off >= len(clx) || clx[off] != 0x02 {
		return nil, fmt.Errorf("expected Pcdt (clxt=0x02) at byte %d, got 0x%02X",
			off, safeByteAt(clx, off))
	}
	off++

	if off+4 > len(clx) {
		return nil, fmt.Errorf("Pcdt truncated before lcb field")
	}
	lcb := int(binary.LittleEndian.Uint32(clx[off:]))
	off += 4

	if off+lcb > len(clx) {
		return nil, fmt.Errorf("PlcPcd extends beyond Clx buffer "+
			"(need %d bytes at offset %d, buffer is %d bytes)", lcb, off, len(clx))
	}
	plcpcd := clx[off : off+lcb]

	if lcb < 4 || (lcb-4)%12 != 0 {
		return nil, fmt.Errorf("PlcPcd size %d inconsistent with formula 12n+4", lcb)
	}
	n := (lcb - 4) / 12

	// Guard against a crafted lcb that would force a multi-gigabyte allocation.
	if n > maxPieceCount {
		return nil, errLimit("piece count", maxPieceCount)
	}

	cps := make([]uint32, n+1)
	for i := range cps {
		cps[i] = binary.LittleEndian.Uint32(plcpcd[i*4:])
	}

	pcdBase := (n + 1) * 4
	pieces := make([]piece, n)
	for i := range n {
		pcd := plcpcd[pcdBase+i*8:]
		fcRaw := binary.LittleEndian.Uint32(pcd[2:])
		fCompressed := (fcRaw>>30)&1 == 1
		fc := fcRaw &^ (3 << 30)
		if fCompressed {
			fc >>= 1
		}
		pieces[i] = piece{
			cpStart:    cps[i],
			cpEnd:      cps[i+1],
			fc:         fc,
			compressed: fCompressed,
		}
	}
	return pieces, nil
}

// ── Streaming piece writer ────────────────────────────────────────────────────

// writePiece reads one piece from the WordDocument stream via ReaderAt and
// writes the decoded, filtered text to w. No intermediate string is built;
// the piece bytes flow directly from the reader through the decoder to the writer.
func writePiece(wd io.ReaderAt, w io.Writer, fc uint32, compressed bool, nChars uint32) error {
	if compressed {
		// 1 byte per character (Windows-1252).
		// Guard: nChars is a uint32; casting to int is safe on 64-bit, but we
		// also enforce the per-piece cap before any allocation.
		if nChars > maxPieceBytes {
			return errLimit("compressed piece size", maxPieceBytes)
		}
		buf := make([]byte, nChars)
		if _, err := wd.ReadAt(buf, int64(fc)); err != nil && err != io.EOF {
			return fmt.Errorf("read compressed piece at fc=%d: %w", fc, err)
		}
		return writeW1252(w, buf)
	}

	// 2 bytes per character (UTF-16LE).
	// Overflow guard: nChars*2 must not wrap on 32-bit platforms and must not
	// exceed the per-piece cap before we allocate.
	if uint64(nChars)*2 > maxPieceBytes {
		return errLimit("unicode piece size", maxPieceBytes)
	}
	byteLen := int(nChars) * 2
	buf := make([]byte, byteLen)
	if _, err := wd.ReadAt(buf, int64(fc)); err != nil && err != io.EOF {
		return fmt.Errorf("read unicode piece at fc=%d: %w", fc, err)
	}
	return writeUTF16LE(w, buf)
}

// writeW1252 decodes Windows-1252 bytes and writes filtered runes to w.
func writeW1252(w io.Writer, b []byte) error {
	// Encode into a small stack buffer to reduce write syscall count.
	var buf [256]byte
	pos := 0
	flush := func() error {
		if pos == 0 {
			return nil
		}
		_, err := w.Write(buf[:pos])
		pos = 0
		return err
	}

	for _, c := range b {
		r := filterRune(w1252Rune(c))
		if r == 0 {
			continue
		}
		// UTF-8 encode r into buf; flush when close to full.
		var tmp [4]byte
		n := encodeRune(tmp[:], r)
		if pos+n > len(buf) {
			if err := flush(); err != nil {
				return err
			}
		}
		copy(buf[pos:], tmp[:n])
		pos += n
	}
	return flush()
}

// writeUTF16LE decodes a UTF-16LE byte slice and writes filtered runes to w.
func writeUTF16LE(w io.Writer, b []byte) error {
	nChars := len(b) / 2
	u16 := make([]uint16, nChars)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	runes := utf16.Decode(u16)

	var buf [256]byte
	pos := 0
	flush := func() error {
		if pos == 0 {
			return nil
		}
		_, err := w.Write(buf[:pos])
		pos = 0
		return err
	}

	for _, r := range runes {
		r = filterRune(r)
		if r == 0 {
			continue
		}
		var tmp [4]byte
		n := encodeRune(tmp[:], r)
		if pos+n > len(buf) {
			if err := flush(); err != nil {
				return err
			}
		}
		copy(buf[pos:], tmp[:n])
		pos += n
	}
	return flush()
}

// encodeRune encodes a single Unicode code point as UTF-8 into p (which must
// be at least 4 bytes) and returns the number of bytes written.
// This avoids importing unicode/utf8 just for EncodeRune.
func encodeRune(p []byte, r rune) int {
	switch {
	case r < 0x80:
		p[0] = byte(r)
		return 1
	case r < 0x800:
		p[0] = byte(0xC0 | r>>6)
		p[1] = byte(0x80 | r&0x3F)
		return 2
	case r < 0x10000:
		p[0] = byte(0xE0 | r>>12)
		p[1] = byte(0x80 | r>>6&0x3F)
		p[2] = byte(0x80 | r&0x3F)
		return 3
	default:
		p[0] = byte(0xF0 | r>>18)
		p[1] = byte(0x80 | r>>12&0x3F)
		p[2] = byte(0x80 | r>>6&0x3F)
		p[3] = byte(0x80 | r&0x3F)
		return 4
	}
}

// ── Character filtering ───────────────────────────────────────────────────────

// filterRune translates or drops Word-specific control characters.
//
// Translated:
//
//	U+0004  column break                     → '\n'
//	U+0007  table cell / row mark            → '\t'
//	U+0009  horizontal tab                   → '\t'
//	U+0014  field separator                  → ' '
//	U+000A  line feed                        → '\n'
//	U+000B  vertical tab / manual line break → '\n'
//	U+000C  page break / section break       → '\n'
//	U+000D  paragraph mark                   → '\n'
//	≥U+0020 printable characters             → as-is
//
// Dropped (returns 0):
//
//	U+0000  NUL
//	U+0001  picture / OLE object anchor
//	U+0002  auto-numbered footnote marker
//	U+0003  end-of-column mark
//	U+0005  annotation reference mark
//	U+0006  footnote / endnote reference mark
//	U+0008  drawn object / frame anchor
//	U+0013  field begin
//	U+0015  field end
//	other control characters < U+0020
func filterRune(r rune) rune {
	switch r {
	case 0x0004:
		return '\n'
	case 0x0007:
		return '\t'
	case 0x0009:
		return '\t'
	case 0x0014:
		return ' '
	case 0x000A, 0x000B, 0x000C, 0x000D:
		return '\n'
	default:
		if r >= 0x0020 {
			return r
		}
		return 0
	}
}

// ── Windows-1252 decoder ──────────────────────────────────────────────────────

// w1252Rune maps a Windows-1252 byte to its Unicode code point.
func w1252Rune(b byte) rune {
	if b < 0x80 || b >= 0xA0 {
		return rune(b)
	}
	var ext = [32]rune{
		/* 80 */ 0x20AC /* 81 */, 0,
		/* 82 */ 0x201A /* 83 */, 0x0192,
		/* 84 */ 0x201E /* 85 */, 0x2026,
		/* 86 */ 0x2020 /* 87 */, 0x2021,
		/* 88 */ 0x02C6 /* 89 */, 0x2030,
		/* 8A */ 0x0160 /* 8B */, 0x2039,
		/* 8C */ 0x0152 /* 8D */, 0,
		/* 8E */ 0x017D /* 8F */, 0,
		/* 90 */ 0 /* 91 */, 0x2018,
		/* 92 */ 0x2019 /* 93 */, 0x201C,
		/* 94 */ 0x201D /* 95 */, 0x2022,
		/* 96 */ 0x2013 /* 97 */, 0x2014,
		/* 98 */ 0x02DC /* 99 */, 0x2122,
		/* 9A */ 0x0161 /* 9B */, 0x203A,
		/* 9C */ 0x0153 /* 9D */, 0,
		/* 9E */ 0x017E /* 9F */, 0x0178,
	}
	return ext[b-0x80]
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func le16(b []byte, off int) uint16 { return binary.LittleEndian.Uint16(b[off:]) }
func le32(b []byte, off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }

func safeByteAt(b []byte, off int) byte {
	if off < len(b) {
		return b[off]
	}
	return 0xFF
}
