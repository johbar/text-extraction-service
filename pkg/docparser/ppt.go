package docparser

// ppt.go — text extraction from PowerPoint Binary Format (.ppt) files.
//
// Algorithm (per [MS-PPT] §2.1.2 PowerPoint Document Stream):
//
//  1. Read CurrentUserAtom from the "Current User" stream.
//     → gives offsetToCurrentEdit into the PowerPoint Document stream.
//
//  2. Walk the UserEditAtom chain (linked list via offsetLastEdit) to build
//     a complete persist object directory: a map of persistId → stream offset.
//     Later edits (closer to end of stream) take precedence over earlier ones.
//
//  3. Look up docPersistIdRef from the first (newest) UserEditAtom to find
//     the DocumentContainer, then parse its SlideListWithTextContainer to get
//     the ordered list of SlidePersistAtom records and their persistIdRefs.
//
//  4. For each slide, seek to its persist offset and walk the record tree
//     rooted at SlideContainer, collecting every TextCharsAtom (0x0FA0) and
//     TextBytesAtom (0x0FA8) encountered. Call the user-supplied callback
//     after each slide.
//
// Record header layout ([MS-PPT] §2.3.1):
//   bits [15:12] recVer      (4 bits) — 0xF means container
//   bits [11: 0] recInstance (12 bits)
//   [2:4] recType  uint16
//   [4:8] recLen   uint32     — bytes following the 8-byte header
//
// Text encoding:
//   TextCharsAtom — UTF-16LE array, length = recLen bytes (recLen/2 chars)
//   TextBytesAtom — Windows-1252 bytes,   length = recLen bytes
//
// Both record types use the same paragraph separator as Word: 0x0D ('\r'),
// which we translate to '\n'. Other Word control characters (fields, object
// anchors) do not appear in PPT text runs, so the filter table is simpler.
//
// References:
//   [MS-PPT] §2.1.1  Current User Stream
//   [MS-PPT] §2.1.2  PowerPoint Document Stream
//   [MS-PPT] §2.3.1  RecordHeader
//   [MS-PPT] §2.3.2  CurrentUserAtom
//   [MS-PPT] §2.3.3  UserEditAtom
//   [MS-PPT] §2.3.4  PersistDirectoryAtom / PersistDirectoryEntry
//   [MS-PPT] §2.4.1  DocumentContainer
//   [MS-PPT] §2.4.14.3 SlideListWithTextContainer / SlidePersistAtom
//   [MS-PPT] §2.5.1  SlideContainer
//   [MS-PPT] §2.9.42 TextCharsAtom  (recType 0x0FA0)
//   [MS-PPT] §2.9.43 TextBytesAtom  (recType 0x0FA8)

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

// ── Public API ────────────────────────────────────────────────────────────────

// SlideText holds the extracted plain text for a single presentation slide.
type SlideText struct {
	// Text contains all text runs from the slide's shapes, joined with newlines.
	// The final newline of a paragraph is normalised to '\n'.
	Text string
	// SlideNumber is 1-based.
	SlideNumber int
}

// ── Record header ─────────────────────────────────────────────────────────────

// Record type constants we care about.
const (
	rtUserEditAtom         = 0x0FF5
	rtPersistDirectoryAtom = 0x1772
	rtDocumentContainer    = 0x03E8
	rtSlideContainer       = 0x03EE
	rtSlideListWithText    = 0x0FF0 // SlideListWithTextContainer
	rtSlidePersistAtom     = 0x03F3
	rtTextCharsAtom        = 0x0FA0
	rtTextBytesAtom        = 0x0FA8
	rtCurrentUserAtom      = 0x0FF6
)

type recHeader struct {
	recVer      uint8  // 4 bits
	recInstance uint16 // 12 bits
	recType     uint16
	recLen      uint32
}

// readHeader reads an 8-byte record header from buf at offset off.
func readHeader(buf []byte, off int) (recHeader, bool) {
	if off+8 > len(buf) {
		return recHeader{}, false
	}
	verInst := binary.LittleEndian.Uint16(buf[off:])
	h := recHeader{
		recVer:      uint8(verInst & 0x0F),
		recInstance: verInst >> 4,
		recType:     binary.LittleEndian.Uint16(buf[off+2:]),
		recLen:      binary.LittleEndian.Uint32(buf[off+4:]),
	}
	return h, true
}

// isContainer returns true when recVer == 0xF (the container sentinel).
func (h recHeader) isContainer() bool { return h.recVer == 0xF }

// ── Persist object directory ──────────────────────────────────────────────────

// buildPersistDir follows the UserEditAtom chain and constructs the complete
// persist object directory as specified in [MS-PPT] §2.1.2 Part 1.
//
// Returns: persistDir (persistId→streamOffset), newestUserEditOffset, error.
func buildPersistDir(doc []byte, offsetToCurrentEdit uint32) (map[uint32]uint32, uint32, error) {
	// We collect (PersistDirectoryAtom offset, UserEditAtom) pairs, then
	// replay them oldest-first so newer edits override older ones.
	type editEntry struct {
		persistDirOffset uint32
	}
	var chain []editEntry

	off := int(offsetToCurrentEdit)
	newestOff := uint32(off)

	for {
		// Guard against a crafted offsetLastEdit chain that loops indefinitely.
		if len(chain) >= maxUserEditChain {
			return nil, 0, errLimit("UserEditAtom chain length", maxUserEditChain)
		}

		h, ok := readHeader(doc, off)
		if !ok || h.recType != rtUserEditAtom {
			return nil, 0, fmt.Errorf("expected UserEditAtom at offset %d, got recType=0x%04X", off, h.recType)
		}
		// UserEditAtom body [MS-PPT §2.3.3]:
		//   [0:4]  lastSlideIdRef   uint32
		//   [4:6]  version          uint16 (ignored)
		//   [6:7]  minorVersion     uint8
		//   [7:8]  majorVersion     uint8
		//   [8:12] offsetLastEdit   uint32
		//   [12:16] offsetPersistDirectory uint32
		//   [16:20] docPersistIdRef uint32
		//   ...

		// Validate that the body is within the buffer before slicing.
		bodyEnd := off + 8 + int(h.recLen)
		if bodyEnd > len(doc) || int(h.recLen) < 20 {
			return nil, 0, fmt.Errorf("UserEditAtom too short or truncated at offset %d", off)
		}
		body := doc[off+8 : bodyEnd]

		offsetLastEdit := binary.LittleEndian.Uint32(body[8:])
		offsetPersistDirectory := binary.LittleEndian.Uint32(body[12:])

		chain = append(chain, editEntry{persistDirOffset: offsetPersistDirectory})

		if offsetLastEdit == 0 {
			break
		}
		// Guard against an offset pointing back into already-processed territory
		// (which would imply a cycle even if the offsets differ each time).
		if int(offsetLastEdit) >= off {
			// offsetLastEdit must point earlier in the stream (older edit).
			// A forward or equal pointer is malformed; stop the chain here
			// rather than risking an infinite loop.
			break
		}
		off = int(offsetLastEdit)
	}

	// Replay oldest-first (chain is newest-first, so reverse).
	// Newer entries override older ones for the same persistId.
	persistDir := make(map[uint32]uint32)
	for i := len(chain) - 1; i >= 0; i-- {
		if err := parsePersistDirAtom(doc, int(chain[i].persistDirOffset), persistDir); err != nil {
			return nil, 0, err
		}
	}
	return persistDir, newestOff, nil
}

// parsePersistDirAtom parses one PersistDirectoryAtom and merges its entries
// into persistDir. [MS-PPT] §2.3.4 / §2.3.5 PersistDirectoryEntry.
//
// PersistDirectoryEntry is a compressed structure:
//
//	bits [19:0]  persistId   — starting persist object identifier
//	bits [31:20] cPersist    — number of PersistOffsetEntry values that follow
//
// Followed by cPersist × uint32 stream offsets.
func parsePersistDirAtom(doc []byte, off int, persistDir map[uint32]uint32) error {
	h, ok := readHeader(doc, off)
	if !ok || h.recType != rtPersistDirectoryAtom {
		return fmt.Errorf("expected PersistDirectoryAtom at offset %d, got recType=0x%04X", off, h.recType)
	}
	// Validate that the body is within the buffer before slicing.
	bodyEnd := off + 8 + int(h.recLen)
	if bodyEnd > len(doc) {
		return fmt.Errorf("PersistDirectoryAtom body extends beyond stream at offset %d", off)
	}
	body := doc[off+8 : bodyEnd]
	pos := 0
	for pos+4 <= len(body) {
		entry := binary.LittleEndian.Uint32(body[pos:])
		pos += 4
		persistId := entry & 0x000FFFFF
		cPersist := entry >> 20
		for i := uint32(0); i < cPersist; i++ {
			if pos+4 > len(body) {
				return fmt.Errorf("PersistDirectoryAtom truncated at pos %d", pos)
			}
			// Guard against unbounded map growth from a crafted file.
			if len(persistDir) >= maxPersistDirEntries {
				return errLimit("persist directory entries", maxPersistDirEntries)
			}
			streamOffset := binary.LittleEndian.Uint32(body[pos:])
			pos += 4
			persistDir[persistId+i] = streamOffset
		}
	}
	return nil
}

// ── Document and slide discovery ──────────────────────────────────────────────

// docPersistIdRef reads docPersistIdRef from the newest UserEditAtom.
func docPersistIdRef(doc []byte, newestUserEditOff uint32) (uint32, error) {
	h, ok := readHeader(doc, int(newestUserEditOff))
	if !ok || h.recType != rtUserEditAtom {
		return 0, fmt.Errorf("UserEditAtom not found at offset %d", newestUserEditOff)
	}
	body := doc[int(newestUserEditOff)+8:]
	if len(body) < 20 {
		return 0, fmt.Errorf("UserEditAtom body too short")
	}
	return binary.LittleEndian.Uint32(body[16:]), nil
}

// slideOffsets returns the stream offsets of all presentation slides in
// presentation order by parsing DocumentContainer → all
// SlideListWithTextContainer children → SlidePersistAtom entries.
//
// Two bugs were found against a real file and fixed here:
//
// Bug 1 — Multiple SlideListWithTextContainer records.
//
//	DocumentContainer can contain several 0x0FF0 children (one each for master
//	slides, presentation slides, and notes). Some writers set recInstance
//	unexpectedly (e.g. recInstance=0 for the presentation slide list instead of
//	the expected 1), so we cannot rely on recInstance to pick the right one.
//	Fix: scan ALL 0x0FF0 children and collect SlidePersistAtom entries from
//	every one of them.
//
// Bug 2 — Master slides mixed in with presentation slides.
//
//	SlidePersistAtom entries may point to MasterOrSlideContainer (0x03F8) or
//	NotesContainer (0x03F0) instead of SlideContainer (0x03EE).
//	Fix: after resolving each persistIdRef to a stream offset, read the record
//	header at that offset and keep only entries whose recType is
//	rtSlideContainer (0x03EE).
func slideOffsets(doc []byte, docOffset uint32, persistDir map[uint32]uint32) ([]uint32, error) {
	off := int(docOffset)
	dh, ok := readHeader(doc, off)
	if !ok || dh.recType != rtDocumentContainer {
		return nil, fmt.Errorf("expected DocumentContainer at offset %d", off)
	}
	// Guard against a crafted recLen that overflows the buffer index.
	if uint64(off)+8+uint64(dh.recLen) > uint64(len(doc)) {
		return nil, fmt.Errorf("DocumentContainer recLen exceeds stream at offset %d", off)
	}

	// Walk every direct child of DocumentContainer.
	end := off + 8 + int(dh.recLen)
	cursor := off + 8
	var offsets []uint32
	for cursor+8 <= end {
		h, ok := readHeader(doc, cursor)
		if !ok {
			break
		}
		if h.recType == rtSlideListWithText {
			offs, err := parseSlidePersistAtoms(doc, cursor, persistDir)
			if err != nil {
				return nil, err
			}
			offsets = append(offsets, offs...)
		}
		// Guard against zero-length records stalling the loop.
		step := 8 + int(h.recLen)
		if step <= 0 {
			break
		}
		cursor += step
	}
	return offsets, nil
}

// parseSlidePersistAtoms walks one SlideListWithTextContainer and returns the
// stream offset of each SlidePersistAtom whose target is a SlideContainer.
//
// SlidePersistAtom body [MS-PPT §2.4.14.5]:
//
//	[0:4]  persistIdRef  uint32
//	[4:8]  flags         uint32
//	[8:12] slideId       uint32
//	...
//
// We resolve each persistIdRef through the persist directory and check the
// recType at the resolved offset: only rtSlideContainer (0x03EE) entries are
// kept. This correctly excludes master slides (0x03F8) and notes (0x03F0).
func parseSlidePersistAtoms(doc []byte, off int, persistDir map[uint32]uint32) ([]uint32, error) {
	h, ok := readHeader(doc, off)
	if !ok {
		return nil, fmt.Errorf("truncated SlideListWithTextContainer header")
	}
	if uint64(off)+8+uint64(h.recLen) > uint64(len(doc)) {
		return nil, fmt.Errorf("SlideListWithTextContainer recLen exceeds stream at offset %d", off)
	}
	end := off + 8 + int(h.recLen)
	cursor := off + 8
	var offsets []uint32
	for cursor+8 <= end {
		ch, ok := readHeader(doc, cursor)
		if !ok {
			break
		}
		if ch.recType == rtSlidePersistAtom {
			body := doc[cursor+8:]
			if len(body) >= 4 {
				persistIdRef := binary.LittleEndian.Uint32(body[:4])
				if streamOff, found := persistDir[persistIdRef]; found {
					if th, ok := readHeader(doc, int(streamOff)); ok && th.recType == rtSlideContainer {
						offsets = append(offsets, streamOff)
					}
				}
			}
		}
		step := 8 + int(ch.recLen)
		if step <= 0 {
			break
		}
		cursor += step
	}
	return offsets, nil
}

// ── Slide text extraction ─────────────────────────────────────────────────────

// extractSlideText recursively walks the record tree rooted at the given
// offset, collecting all TextCharsAtom and TextBytesAtom payloads.
//
// end is the exclusive upper bound for this call — the byte offset one past
// the last byte of the enclosing container's body. Each recursive call passes
// its own bodyEnd as the end for the child, so we never walk outside the
// boundary of the container we are currently processing.
//
// Without this bound, every recursive call would use len(doc) as its end,
// causing exponential blowup: a container near the end of the stream would
// spawn a full re-walk of all remaining records (including other slides and
// persist-directory atoms), and each of those containers would do the same,
// resulting in millions of calls and an effectively infinite runtime.
func extractSlideText(doc []byte, off, end int, sb *strings.Builder) {
	cursor := off
	for cursor+8 <= end {
		h, ok := readHeader(doc, cursor)
		if !ok {
			break
		}
		bodyOff := cursor + 8
		bodyEnd := bodyOff + int(h.recLen)
		if bodyEnd > end {
			break // record extends past our container boundary — stop
		}

		switch h.recType {
		case rtTextCharsAtom:
			// UTF-16LE array, recLen bytes → recLen/2 uint16 code units.
			// Guard: recLen is uint32; bodyEnd-bodyOff is already bounded by
			// the container's end, but we also cap the allocation explicitly.
			if h.recLen > maxPieceBytes {
				// Oversized text atom in a legitimate file is implausible; skip it.
				cursor = bodyEnd
				continue
			}
			body := doc[bodyOff:bodyEnd]
			n := len(body) / 2
			u16 := make([]uint16, n)
			for i := range u16 {
				u16[i] = binary.LittleEndian.Uint16(body[i*2:])
			}
			runes := utf16.Decode(u16)
			for _, r := range runes {
				switch r {
				case '\r', 0x000B, 0x000C:
					sb.WriteByte('\n')
				case 0x0000:
					// NUL is forbidden by spec but tolerate it
				default:
					if r >= 0x0020 {
						sb.WriteRune(r)
					}
				}
			}
			sb.WriteByte('\n') // separate text runs

		case rtTextBytesAtom:
			// Windows-1252 byte array.
			if h.recLen > maxPieceBytes {
				cursor = bodyEnd
				continue
			}
			body := doc[bodyOff:bodyEnd]
			for _, b := range body {
				switch b {
				case '\r', 0x0B, 0x0C:
					sb.WriteByte('\n')
				case 0x00:
					// skip
				default:
					if r := w1252Rune(b); r != 0 && r >= 0x0020 {
						sb.WriteRune(r)
					}
				}
			}
			sb.WriteByte('\n')

		default:
			// Recurse into containers (recVer == 0xF), bounded to this container.
			if h.isContainer() {
				extractSlideText(doc, bodyOff, bodyEnd, sb)
			}
		}

		cursor = bodyEnd
	}
}

// ── Top-level orchestration ───────────────────────────────────────────────────

func extractSlides(pptDoc, currentUser []byte, fn func(SlideText) error) error {
	// ── 1. Read CurrentUserAtom ───────────────────────────────────────────────
	//
	// CurrentUserAtom [MS-PPT §2.3.2]:
	//   rh (8 bytes): RecordHeader, recType = 0x0FF6
	//   size      (4 bytes): MUST be 0x14
	//   headerToken (4 bytes): MUST be 0xE391C05F (unencrypted)
	//   offsetToCurrentEdit (4 bytes): offset into PowerPoint Document stream
	//   ... more fields we skip
	if len(currentUser) < 20 {
		return fmt.Errorf("Current User stream too short (%d bytes)", len(currentUser))
	}
	cuH, ok := readHeader(currentUser, 0)
	if !ok || cuH.recType != rtCurrentUserAtom {
		return fmt.Errorf("expected CurrentUserAtom, got recType=0x%04X", cuH.recType)
	}
	// Body starts at offset 8.
	cuBody := currentUser[8:]
	if len(cuBody) < 12 {
		return fmt.Errorf("CurrentUserAtom body too short")
	}
	headerToken := binary.LittleEndian.Uint32(cuBody[4:])
	if headerToken == 0xDFC4D1F3 {
		return fmt.Errorf("presentation is encrypted; cannot extract text")
	}
	offsetToCurrentEdit := binary.LittleEndian.Uint32(cuBody[8:])

	// ── 2. Build persist object directory ────────────────────────────────────
	persistDir, newestUEOff, err := buildPersistDir(pptDoc, offsetToCurrentEdit)
	if err != nil {
		return fmt.Errorf("buildPersistDir: %w", err)
	}

	// ── 3. Find DocumentContainer ─────────────────────────────────────────────
	docPersistId, err := docPersistIdRef(pptDoc, newestUEOff)
	if err != nil {
		return fmt.Errorf("docPersistIdRef: %w", err)
	}
	docOff, found := persistDir[docPersistId]
	if !found {
		return fmt.Errorf("DocumentContainer persistId %d not in persist directory", docPersistId)
	}

	// ── 4. Enumerate slide offsets ────────────────────────────────────────────
	slideOffs, err := slideOffsets(pptDoc, docOff, persistDir)
	if err != nil {
		return fmt.Errorf("slideOffsets: %w", err)
	}

	// ── 5. Extract text slide by slide ────────────────────────────────────────
	for i, off := range slideOffs {
		// Guard against an implausibly large number of slides.
		if i >= maxSlides {
			return errLimit("slide count", maxSlides)
		}
		if int(off)+8 > len(pptDoc) {
			continue
		}
		sh, ok := readHeader(pptDoc, int(off))
		if !ok || sh.recType != rtSlideContainer {
			continue // not a slide container, skip
		}
		// Guard against a crafted recLen that would overflow the buffer index.
		// uint64 arithmetic avoids any 32-bit wrap before the comparison.
		if uint64(off)+8+uint64(sh.recLen) > uint64(len(pptDoc)) {
			continue
		}

		var sb strings.Builder
		slideBodyEnd := int(off) + 8 + int(sh.recLen)
		extractSlideText(pptDoc, int(off)+8, slideBodyEnd, &sb)

		text := sb.String()
		if err := fn(SlideText{SlideNumber: i + 1, Text: text}); err != nil {
			return err
		}
	}

	return nil
}
