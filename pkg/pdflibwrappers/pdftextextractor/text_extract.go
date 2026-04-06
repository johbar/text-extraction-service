/*
Package pdftextextractor implements a text extractor for PDFs ontop of pdfcpu.
It is not on spar with C++ PDF libs like PDFium or Poppler regarding accuracy and quality.
*/
package pdftextextractor

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

var (
	// glyphNames is a subset of the Adobe Glyph List covering the most common names.
	glyphNames = map[string]rune{
		"space": ' ', "exclam": '!', "quotedbl": '"', "numbersign": '#',
		"dollar": '$', "percent": '%', "ampersand": '&', "quotesingle": '\'',
		"parenleft": '(', "parenright": ')', "asterisk": '*', "plus": '+',
		"comma": ',', "hyphen": '-', "period": '.', "slash": '/',
		"zero": '0', "one": '1', "two": '2', "three": '3', "four": '4',
		"five": '5', "six": '6', "seven": '7', "eight": '8', "nine": '9',
		"colon": ':', "semicolon": ';', "less": '<', "equal": '=',
		"greater": '>', "question": '?', "at": '@',
		"A": 'A', "B": 'B', "C": 'C', "D": 'D', "E": 'E', "F": 'F',
		"G": 'G', "H": 'H', "I": 'I', "J": 'J', "K": 'K', "L": 'L',
		"M": 'M', "N": 'N', "O": 'O', "P": 'P', "Q": 'Q', "R": 'R',
		"S": 'S', "T": 'T', "U": 'U', "V": 'V', "W": 'W', "X": 'X',
		"Y": 'Y', "Z": 'Z',
		"bracketleft": '[', "backslash": '\\', "bracketright": ']',
		"asciicircum": '^', "underscore": '_', "grave": '`',
		"a": 'a', "b": 'b', "c": 'c', "d": 'd', "e": 'e', "f": 'f',
		"g": 'g', "h": 'h', "i": 'i', "j": 'j', "k": 'k', "l": 'l',
		"m": 'm', "n": 'n', "o": 'o', "p": 'p', "q": 'q', "r": 'r',
		"s": 's', "t": 't', "u": 'u', "v": 'v', "w": 'w', "x": 'x',
		"y": 'y', "z": 'z',
		"braceleft": '{', "bar": '|', "braceright": '}', "asciitilde": '~',
		// Common extras.
		"endash": '\u2013', "emdash": '\u2014',
		"quoteleft": '\u2018', "quoteright": '\u2019',
		"quotedblleft": '\u201C', "quotedblright": '\u201D',
		"bullet": '\u2022', "ellipsis": '\u2026',
		"trademark": '\u2122', "copyright": '\u00A9', "registered": '\u00AE',
		"fi": '\uFB01', "fl": '\uFB02',
		"AE": '\u00C6', "ae": '\u00E6',
		"OE": '\u0152', "oe": '\u0153',
		"Oslash": '\u00D8', "oslash": '\u00F8',
		"Aacute": '\u00C1', "aacute": '\u00E1',
		"Agrave": '\u00C0', "agrave": '\u00E0',
		"Acircumflex": '\u00C2', "acircumflex": '\u00E2',
		"Atilde": '\u00C3', "atilde": '\u00E3',
		"Adieresis": '\u00C4', "adieresis": '\u00E4',
		"Eacute": '\u00C9', "eacute": '\u00E9',
		"Egrave": '\u00C8', "egrave": '\u00E8',
		"Ecircumflex": '\u00CA', "ecircumflex": '\u00EA',
		"Edieresis": '\u00CB', "edieresis": '\u00EB',
		"Iacute": '\u00CD', "iacute": '\u00ED',
		"Igrave": '\u00CC', "igrave": '\u00EC',
		"Icircumflex": '\u00CE', "icircumflex": '\u00EE',
		"Idieresis": '\u00CF', "idieresis": '\u00EF',
		"Oacute": '\u00D3', "oacute": '\u00F3',
		"Ograve": '\u00D2', "ograve": '\u00F2',
		"Ocircumflex": '\u00D4', "ocircumflex": '\u00F4',
		"Otilde": '\u00D5', "otilde": '\u00F5',
		"Odieresis": '\u00D6', "odieresis": '\u00F6',
		"Uacute": '\u00DA', "uacute": '\u00FA',
		"Ugrave": '\u00D9', "ugrave": '\u00F9',
		"Ucircumflex": '\u00DB', "ucircumflex": '\u00FB',
		"Udieresis": '\u00DC', "udieresis": '\u00FC',
		"Ntilde": '\u00D1', "ntilde": '\u00F1',
		"Ccedilla": '\u00C7', "ccedilla": '\u00E7',
		"Yacute": '\u00DD', "yacute": '\u00FD',
		"Ydieresis": '\u0178', "ydieresis": '\u00FF',
		"germandbls": '\u00DF',
		"degree":     '\u00B0', "multiply": '\u00D7', "divide": '\u00F7',
		"minus": '-', "plusminus": '\u00B1',
		"onehalf": '\u00BD', "onequarter": '\u00BC', "threequarters": '\u00BE',
		"sterling": '\u00A3', "yen": '\u00A5', "Euro": '\u20AC', "cent": '\u00A2',
		"guillemotleft": '\u00AB', "guillemotright": '\u00BB',
		"guilsinglleft": '\u2039', "guilsinglright": '\u203A',
		"dagger": '\u2020', "daggerdbl": '\u2021',
		"section": '\u00A7', "paragraph": '\u00B6',
		"acute": '\u00B4', "dieresis": '\u00A8',
		"circumflex": '\u02C6', "tilde": '\u02DC', "cedilla": '\u00B8',
		"macron": '\u00AF', "breve": '\u02D8', "dotaccent": '\u02D9',
		"ring": '\u02DA', "hungarumlaut": '\u02DD', "ogonek": '\u02DB',
		"caron": '\u02C7', "dotlessi": '\u0131',
		"fraction": '\u2044', "perthousand": '\u2030',
		"mu": '\u00B5', "periodcentered": '\u00B7', "ordmasculine": '\u00BA',
		"ordfeminine": '\u00AA', "questiondown": '\u00BF', "exclamdown": '\u00A1',
		"notsign": '\u00AC', "softhyphen": '\u00AD', "nonbreakingspace": '\u00A0',
		"florin": '\u0192', "lozenge": '\u25CA',
	}

	standardEnc map[byte]rune = map[byte]rune{
		0x20: ' ', 0x21: '!', 0x22: '"', 0x23: '#', 0x24: '$', 0x25: '%',
		0x26: '&', 0x27: '\'', 0x28: '(', 0x29: ')', 0x2A: '*', 0x2B: '+',
		0x2C: ',', 0x2D: '-', 0x2E: '.', 0x2F: '/', 0x30: '0', 0x31: '1',
		0x32: '2', 0x33: '3', 0x34: '4', 0x35: '5', 0x36: '6', 0x37: '7',
		0x38: '8', 0x39: '9', 0x3A: ':', 0x3B: ';', 0x3C: '<', 0x3D: '=',
		0x3E: '>', 0x3F: '?', 0x40: '@', 0x41: 'A', 0x42: 'B', 0x43: 'C',
		0x44: 'D', 0x45: 'E', 0x46: 'F', 0x47: 'G', 0x48: 'H', 0x49: 'I',
		0x4A: 'J', 0x4B: 'K', 0x4C: 'L', 0x4D: 'M', 0x4E: 'N', 0x4F: 'O',
		0x50: 'P', 0x51: 'Q', 0x52: 'R', 0x53: 'S', 0x54: 'T', 0x55: 'U',
		0x56: 'V', 0x57: 'W', 0x58: 'X', 0x59: 'Y', 0x5A: 'Z', 0x5B: '[',
		0x5C: '\\', 0x5D: ']', 0x5E: '^', 0x5F: '_', 0x60: '`', 0x61: 'a',
		0x62: 'b', 0x63: 'c', 0x64: 'd', 0x65: 'e', 0x66: 'f', 0x67: 'g',
		0x68: 'h', 0x69: 'i', 0x6A: 'j', 0x6B: 'k', 0x6C: 'l', 0x6D: 'm',
		0x6E: 'n', 0x6F: 'o', 0x70: 'p', 0x71: 'q', 0x72: 'r', 0x73: 's',
		0x74: 't', 0x75: 'u', 0x76: 'v', 0x77: 'w', 0x78: 'x', 0x79: 'y',
		0x7A: 'z',
		// Common Adobe Standard extras.
		0x91: '\u2018', 0x92: '\u2019', 0x93: '\u201C', 0x94: '\u201D',
		0x96: '\u2013', 0x97: '\u2014', 0xA0: '\u00A0',
		0xAD: '\u00AD', 0xC6: '\u00C6', 0xE6: '\u00E6',
	}

	macRomanEnc map[byte]rune
	winAnsiEnc  map[byte]rune
)

func init() {
	macRomanEnc = macRomanEncoding()
	winAnsiEnc = winAnsiEncoding()
}

// extractPageText extracts the readable text from the given page of ctx.
// pageNr is 1-based. Returns a bytes.Buffer with the decoded text or an error.
func extractPageText(ctx *model.Context, pageNr int) (*bytes.Buffer, error) {
	if pageNr < 1 || pageNr > ctx.PageCount {
		return nil, fmt.Errorf("extractPageText: invalid page number %d (document has %d pages)", pageNr, ctx.PageCount)
	}

	consolidateRes := true
	pageDict, _, inhPAttrs, err := ctx.XRefTable.PageDict(pageNr, consolidateRes)
	if err != nil {
		return nil, fmt.Errorf("extractPageText: page %d: %w", pageNr, err)
	}
	if pageDict == nil {
		return nil, fmt.Errorf("extractPageText: page %d not found", pageNr)
	}

	content, err := ctx.XRefTable.PageContent(pageDict, pageNr)
	if err != nil {
		if err == model.ErrNoContent {
			return nil, nil
		}
		return nil, fmt.Errorf("extractPageText: page %d content: %w", pageNr, err)
	}

	fontMap := buildFontMap(ctx.XRefTable, inhPAttrs.Resources)

	text, err := extractTextFromContent(content, fontMap)
	if err != nil {
		return nil, fmt.Errorf("extractPageText: page %d parse: %w", pageNr, err)
	}

	return &text, nil
}

// ---------------------------------------------------------------------------
// Font encoding and width helpers
// ---------------------------------------------------------------------------

// pdfFont holds just enough font metadata to decode text operands and compute
// glyph advance widths for cursor tracking.
type pdfFont struct {
	// encoding maps a single-byte character code to a Unicode rune.
	// nil means fall back to Latin-1.
	encoding map[byte]rune

	// toUnicode maps a character code (big-endian uint16) to a Unicode string.
	// Takes precedence over encoding when present.
	toUnicode map[uint16]string

	// widths maps a character code to its advance width in PDF glyph-space
	// units (1/1000 of a text unit). Covers both 1-byte (simple fonts) and
	// 2-byte (CIDFont) codes stored as uint16.
	widths map[uint16]float64

	// defaultWidth is used for codes absent from widths. Overridden by the
	// FontDescriptor's MissingWidth; falls back to 500 (half an em).
	defaultWidth float64
}

// glyphAdvance returns the advance width for the glyph encoded at b[i] in
// glyph-space units (1/1000 text unit), together with the number of bytes
// consumed (1 for simple fonts, 2 for composite fonts using 2-byte codes).
func (f *pdfFont) glyphAdvance(b []byte, i int) (width float64, consumed int) {
	dw := f.defaultWidth
	if dw == 0 {
		dw = 500
	}
	if f.widths == nil {
		return dw, 1
	}
	// Try 2-byte code first (composite / CIDFont).
	if i+1 < len(b) {
		if w, ok := f.widths[(uint16(b[i])<<8)|uint16(b[i+1])]; ok {
			return w, 2
		}
	}
	// 1-byte code (simple font).
	if w, ok := f.widths[uint16(b[i])]; ok {
		return w, 1
	}
	return dw, 1
}

// rawStringWidth returns the total advance of raw glyph bytes b in glyph-space
// units, without scaling by font size.
func (f *pdfFont) rawStringWidth(b []byte) float64 {
	if f == nil {
		return float64(len(b)) * 500
	}
	total := 0.0
	for i := 0; i < len(b); {
		w, n := f.glyphAdvance(b, i)
		total += w
		i += n
	}
	return total
}

// decodeBytes converts raw character-code bytes to UTF-8 using the font's
// encoding tables.
func (f *pdfFont) decodeBytes(b []byte) []byte {
	var sb bytes.Buffer
	for i := 0; i < len(b); {
		if f.toUnicode != nil && i+1 < len(b) {
			code := (uint16(b[i]) << 8) | uint16(b[i+1])
			if s, ok := f.toUnicode[code]; ok {
				sb.WriteString(s)
				i += 2
				continue
			}
		}
		if f.toUnicode != nil {
			if s, ok := f.toUnicode[uint16(b[i])]; ok {
				sb.WriteString(s)
				i++
				continue
			}
		}
		if f.encoding != nil {
			if r, ok := f.encoding[b[i]]; ok {
				sb.WriteRune(r)
				i++
				continue
			}
		}
		r := rune(b[i])
		if r >= 0x20 && r != 0x7f {
			sb.WriteRune(r)
		}
		i++
	}
	return sb.Bytes()
}

// buildFontMap inspects a page resource dict and returns a map from PDF font
// resource name (e.g. "F1") to a *pdfFont.
func buildFontMap(xRefTable *model.XRefTable, resources types.Dict) map[string]*pdfFont {
	fontMap := make(map[string]*pdfFont)
	if resources == nil {
		return fontMap
	}

	obj, found := resources.Find("Font")
	if !found {
		return fontMap
	}

	fontDict, err := xRefTable.DereferenceDict(obj)
	if err != nil || fontDict == nil {
		return fontMap
	}

	for name, ref := range fontDict {
		fd, err := xRefTable.DereferenceDict(ref)
		if err != nil || fd == nil {
			continue
		}
		f := &pdfFont{defaultWidth: 500}

		// ToUnicode CMap.
		if tuRef, ok := fd.Find("ToUnicode"); ok {
			if sd, _, err := xRefTable.DereferenceStreamDict(tuRef); err == nil && sd != nil {
				if err := sd.Decode(); err == nil {
					f.toUnicode = parseToUnicodeCMap(sd.Content)
				}
			}
		}

		// Encoding.
		if encObj, ok := fd.Find("Encoding"); ok {
			f.encoding = parseEncoding(xRefTable, encObj)
		}

		// Glyph widths. Type0 (composite) fonts store widths in the
		// descendant CIDFont dict under the W key; all other font types
		// use the FirstChar/LastChar/Widths triplet.
		subtype, _ := fd.Find("Subtype")
		if n, ok := subtype.(types.Name); ok && n.Value() == "Type0" {
			f.widths = parseCIDFontWidths(xRefTable, fd)
		} else {
			f.widths = parseSimpleFontWidths(xRefTable, fd)
		}

		// MissingWidth from FontDescriptor overrides the default.
		if fdRef, ok := fd.Find("FontDescriptor"); ok {
			if fdDict, err := xRefTable.DereferenceDict(fdRef); err == nil && fdDict != nil {
				if mw, ok := fdDict.Find("MissingWidth"); ok {
					switch v := mw.(type) {
					case types.Integer:
						f.defaultWidth = float64(v.Value())
					case types.Float:
						f.defaultWidth = v.Value()
					}
				}
			}
		}

		fontMap[name] = f
	}

	return fontMap
}

// parseSimpleFontWidths extracts the Widths array from a simple font dict
// (Type1, TrueType, Type3) using the FirstChar/LastChar/Widths triplet.
func parseSimpleFontWidths(xRefTable *model.XRefTable, fd types.Dict) map[uint16]float64 {
	fcObj, ok1 := fd.Find("FirstChar")
	wObj, ok2 := fd.Find("Widths")
	if !ok1 || !ok2 {
		return nil
	}
	fc, ok := fcObj.(types.Integer)
	if !ok {
		return nil
	}
	arr, err := xRefTable.DereferenceArray(wObj)
	if err != nil || len(arr) == 0 {
		return nil
	}
	widths := make(map[uint16]float64, len(arr))
	for idx, entry := range arr {
		switch v := entry.(type) {
		case types.Integer:
			widths[uint16(fc.Value()+idx)] = float64(v.Value())
		case types.Float:
			widths[uint16(fc.Value()+idx)] = v.Value()
		}
	}
	return widths
}

// parseCIDFontWidths extracts glyph widths from the W array of a Type0 font's
// descendant CIDFont dict. The W array uses two alternate forms:
//
//	c [w0 w1 … wn-1]   — individual widths for codes c through c+n-1
//	c1 c2 w             — uniform width w for all codes c1 through c2
func parseCIDFontWidths(xRefTable *model.XRefTable, type0fd types.Dict) map[uint16]float64 {
	dfObj, ok := type0fd.Find("DescendantFonts")
	if !ok {
		return nil
	}
	dfArr, err := xRefTable.DereferenceArray(dfObj)
	if err != nil || len(dfArr) == 0 {
		return nil
	}
	cidfd, err := xRefTable.DereferenceDict(dfArr[0])
	if err != nil || cidfd == nil {
		return nil
	}
	wObj, ok := cidfd.Find("W")
	if !ok {
		return nil
	}
	wArr, err := xRefTable.DereferenceArray(wObj)
	if err != nil || len(wArr) == 0 {
		return nil
	}

	widths := make(map[uint16]float64)
	i := 0
	for i < len(wArr) {
		cObj, ok := wArr[i].(types.Integer)
		if !ok {
			i++
			continue
		}
		c := cObj.Value()
		i++
		if i >= len(wArr) {
			break
		}
		switch next := wArr[i].(type) {
		case types.Array:
			// c [w0 w1 …]
			for j, wEntry := range next {
				switch v := wEntry.(type) {
				case types.Integer:
					widths[uint16(c+j)] = float64(v.Value())
				case types.Float:
					widths[uint16(c+j)] = v.Value()
				}
			}
			i++
		case types.Integer:
			// c1 c2 w
			c2 := next.Value()
			i++
			if i >= len(wArr) {
				break
			}
			var w float64
			switch v := wArr[i].(type) {
			case types.Integer:
				w = float64(v.Value())
			case types.Float:
				w = v.Value()
			}
			for code := c; code <= c2; code++ {
				widths[uint16(code)] = w
			}
			i++
		default:
			i++
		}
	}
	return widths
}

// parseEncoding converts a PDF Encoding entry (Name or Dict) into a char→rune map.
func parseEncoding(xRefTable *model.XRefTable, obj types.Object) map[byte]rune {
	obj, err := xRefTable.Dereference(obj)
	if err != nil || obj == nil {
		return nil
	}
	switch v := obj.(type) {
	case types.Name:
		return namedEncoding(v.Value())
	case types.Dict:
		var base map[byte]rune
		if baseObj, ok := v.Find("BaseEncoding"); ok {
			if n, ok := baseObj.(types.Name); ok {
				base = namedEncoding(n.Value())
			}
		}
		if base == nil {
			base = maps.Clone(standardEnc)
		}
		if diffObj, ok := v.Find("Differences"); ok {
			arr, err := xRefTable.DereferenceArray(diffObj)
			if err == nil {
				applyDifferences(base, arr)
			}
		}
		return base
	}
	return nil
}

// applyDifferences modifies enc in-place using a PDF Differences array.
func applyDifferences(enc map[byte]rune, diffs types.Array) {
	code := 0
	for _, item := range diffs {
		switch v := item.(type) {
		case types.Integer:
			code = v.Value()
		case types.Name:
			if r, ok := glyphToRune(v.Value()); ok {
				enc[byte(code)] = r
			}
			code++
		}
	}
}

// namedEncoding returns a mutable copy of one of the standard PDF named encodings.
func namedEncoding(name string) map[byte]rune {
	switch name {
	case "MacRomanEncoding":
		return maps.Clone(macRomanEnc)
	case "WinAnsiEncoding":
		return maps.Clone(winAnsiEnc)
	default:
		return maps.Clone(standardEnc)
	}
}

// ---------------------------------------------------------------------------
// Content stream parser
// ---------------------------------------------------------------------------

// textState tracks the PDF text state machine during content stream parsing.
type textState struct {
	inBT        bool
	currentFont *pdfFont
	fontMap     map[string]*pdfFont
	wordSpacing float64

	// tlTx/tlTy is the text line matrix origin — the reference point used by
	// Td/TD (relative offsets) to compute the next absolute position.
	// Updated by Tm (absolute set), Td/TD (accumulated relative offset), T*.
	tlTx, tlTy float64
	tlSet      bool // true once we have seen at least one positioning op

	// cursorX/cursorY is the pen position after the last rendered glyph, in
	// user-space coordinates. This is what we compare against the next
	// operator's target position to decide whether to emit a space or newline.
	//
	// cursorX advances after each text-showing operator by the sum of glyph
	// advances scaled to user space: advance = (glyphWidth/1000) * fontSize.
	// cursorY mirrors the current baseline and only changes on line moves.
	cursorX, cursorY float64

	// leading is the current text leading (set by TL, implied by TD).
	leading float64

	// fontSize is the current font size in user-space points.
	fontSize float64
}

// reposition sets both the text line origin and the cursor to (x, y) and
// marks the state as having a valid reference position. Called by every
// operator that moves the text position.
func (ts *textState) reposition(x, y float64) {
	ts.tlTx, ts.tlTy = x, y
	ts.cursorX, ts.cursorY = x, y
	ts.tlSet = true
}

// advanceCursor moves cursorX forward by the user-space width of the glyph
// sequence encoded in raw bytes b:
//
//	userSpaceAdvance = (glyphSpaceWidth / 1000) * fontSize
func (ts *textState) advanceCursor(b []byte) {
	if ts.fontSize == 0 {
		return
	}
	var gsWidth float64
	if ts.currentFont != nil {
		gsWidth = ts.currentFont.rawStringWidth(b)
	} else {
		gsWidth = float64(len(b)) * 500
	}
	ts.cursorX += gsWidth / 1000.0 * ts.fontSize
}

// emitGap compares the new text origin (newX, newY) against the current cursor
// position and writes a separator to w when needed:
//
//   - |newY - cursorY| > lineThreshold  →  newline
//   - same line AND newX - cursorX > spaceThreshold  →  space
//   - otherwise  →  nothing
//
// Always calls reposition afterwards so the cursor reflects the new origin.
func (ts *textState) emitGap(w io.ByteWriter, newX, newY float64) {
	if !ts.tlSet {
		ts.reposition(newX, newY)
		return
	}

	lineThreshold := ts.fontSize * 0.5
	if lineThreshold < 1 {
		lineThreshold = 1
	}

	dy := ts.cursorY - newY // positive = moved down the page (PDF y points up)
	if dy > lineThreshold || dy < -lineThreshold {
		w.WriteByte('\n')
	} else {
		// Same baseline — check whether there is a visible gap between where
		// the last glyph ended (cursorX) and where the next chunk starts (newX).
		// Threshold: ~20% of font size. This clears normal kerning adjustments
		// (±50–200 glyph units ≈ ±0.05–0.2 em) while catching word gaps
		// (typically 200–500 glyph units ≈ 0.2–0.5 em).
		spaceThreshold := ts.fontSize * 0.2
		if spaceThreshold < 1 {
			spaceThreshold = 1
		}
		if newX-ts.cursorX > spaceThreshold {
			w.WriteByte(' ')
		}
	}

	ts.reposition(newX, newY)
}

// extractTextFromContent parses a PDF content stream and returns plain text.
func extractTextFromContent(content []byte, fontMap map[string]*pdfFont) (bytes.Buffer, error) {
	var out bytes.Buffer
	ts := &textState{fontMap: fontMap}

	tokens := tokenize(content)
	i := 0
	for i < len(tokens) {
		tok := tokens[i]

		switch string(tok) {
		case "BT":
			ts.inBT = true
			// Reset the text line matrix origin to (0,0) as the PDF spec
			// requires: Td/TD offsets are relative to this origin and it
			// starts fresh for every new text object.
			// cursorX/cursorY are intentionally NOT reset: they carry the
			// pen position across ET/BT boundaries so that emitGap can
			// compare the next chunk position against where the last
			// glyph ended and detect same-line vs new-line correctly.
			ts.tlTx, ts.tlTy = 0, 0

		case "ET":
			ts.inBT = false
			// No newline here. All line-break decisions are made by emitGap
			// when the next positioning operator fires. Emitting \n on every
			// ET would split same-line chunks that happen to be in separate
			// BT blocks (common in tagged PDFs where each word or glyph run
			// gets its own BT/ET pair).

		case "Tf":
			// /FontName size Tf
			if i >= 2 {
				fontName := stripSlash(tokens[i-2])
				if f, ok := fontMap[string(fontName)]; ok {
					ts.currentFont = f
				} else {
					ts.currentFont = nil
				}
				ts.fontSize, _ = strconv.ParseFloat(string(tokens[i-1]), 64)
				if ts.fontSize < 0 {
					ts.fontSize = -ts.fontSize
				}
			}

		case "TL":
			if i >= 1 {
				ts.leading, _ = strconv.ParseFloat(string(tokens[i-1]), 64)
			}

		case "Tw":
			if i >= 1 {
				ts.wordSpacing, _ = strconv.ParseFloat(string(tokens[i-1]), 64)
			}

		case "Td", "TD":
			// tx ty Td — move line position by (tx, ty) relative to current
			// line origin. TD also sets leading = -ty.
			if ts.inBT && i >= 2 {
				dy, errY := strconv.ParseFloat(string(tokens[i-1]), 64)
				dx, errX := strconv.ParseFloat(string(tokens[i-2]), 64)
				if errY == nil && errX == nil {
					if bytes.Equal(tok, []byte{'T', 'D'}) {
						ts.leading = -dy
					}
					ts.emitGap(&out, ts.tlTx+dx, ts.tlTy+dy)
				}
			}

		case "T*":
			// Equivalent to 0 -leading Td.
			if ts.inBT {
				ts.emitGap(&out, 0, ts.tlTy+(-ts.leading))
			}

		case "Tm":
			// a b c d tx ty Tm — absolute position.
			if ts.inBT && i >= 6 {
				newTy, errY := strconv.ParseFloat(string(tokens[i-1]), 64)
				newTx, errX := strconv.ParseFloat(string(tokens[i-2]), 64)
				if errY == nil && errX == nil {
					ts.emitGap(&out, newTx, newTy)
				}
			}

		case "Tj":
			if ts.inBT && i >= 1 {
				raw, ok := parsePDFString(tokens[i-1])
				if ok {
					out.Write(decodeRaw(raw, ts.currentFont))
					ts.advanceCursor(raw)
				}
			}

		case "'":
			// Move to next line then show string.
			if ts.inBT && i >= 1 {
				ts.emitGap(&out, 0, ts.tlTy+(-ts.leading))
				raw, ok := parsePDFString(tokens[i-1])
				if ok {
					out.Write(decodeRaw(raw, ts.currentFont))
					ts.advanceCursor(raw)
				}
			}

		case "\"":
			// aw ac string " — set word/char spacing, move to next line, show.
			if ts.inBT && i >= 3 {
				ts.wordSpacing, _ = strconv.ParseFloat(string(tokens[i-3]), 64)
				// tokens[i-2] is charSpacing (Tc) — not tracked here.
				ts.emitGap(&out, 0, ts.tlTy+(-ts.leading))
				raw, ok := parsePDFString(tokens[i-1])
				if ok {
					out.Write(decodeRaw(raw, ts.currentFont))
					ts.advanceCursor(raw)
				}
			}

		case "TJ":
			// Interleaved strings and kerning numbers.
			if ts.inBT && i >= 1 {
				gsAdvance := decodeTJInto(tokens[i-1], ts.currentFont, &out)
				// Scale from glyph-space to user-space and advance cursor.
				ts.cursorX += gsAdvance / 1000.0 * ts.fontSize
			}
		}

		i++
	}
	out = normaliseWhitespace(out.Bytes())
	return out, nil
}

// decodeRaw decodes raw PDF string bytes to UTF-8 via the font's tables,
// falling back to Latin-1 when no font is active.
func decodeRaw(raw []byte, f *pdfFont) []byte {
	if f == nil {
		return decodeLatin1(raw)
	}
	return f.decodeBytes(raw)
}

// decodeTJInto decodes the array operand of a TJ operator, writes the
// decoded text to w, and returns the net advance in glyph-space units
// (1/1000 text unit). Kerning numbers are subtracted from the advance.
// The caller scales to user-space by multiplying by fontSize/1000.
func decodeTJInto(tok []byte, f *pdfFont, w *bytes.Buffer) float64 {
	tok = bytes.TrimSpace(tok)
	if len(tok) < 2 || tok[0] != '[' || tok[len(tok)-1] != ']' {
		return 0
	}
	inner := tok[1 : len(tok)-1]
	var gsAdvance float64
	i := 0
	for i < len(inner) {
		for i < len(inner) && isWhitespaceByte(inner[i]) {
			i++
		}
		if i >= len(inner) {
			break
		}

		if inner[i] == '(' {
			end := findClosingParen(inner, i)
			if end < 0 {
				break
			}
			raw, ok := parsePDFString(inner[i : end+1])
			if ok {
				w.Write(decodeRaw(raw, f))
				if f != nil {
					gsAdvance += f.rawStringWidth(raw)
				} else {
					gsAdvance += float64(len(raw)) * 500
				}
			}
			i = end + 1

		} else if inner[i] == '<' {
			end := bytes.Index(inner[i:], []byte{'>'})
			if end < 0 {
				break
			}
			raw, ok := parsePDFString(inner[i : i+end+1])
			if ok {
				w.Write(decodeRaw(raw, f))
				if f != nil {
					gsAdvance += f.rawStringWidth(raw)
				} else {
					gsAdvance += float64(len(raw)) * 500
				}
			}
			i += end + 1

		} else {
			// Kerning number — negative values tighten spacing (subtract).
			start := i
			for i < len(inner) && !isWhitespaceByte(inner[i]) && inner[i] != '(' && inner[i] != '<' {
				i++
			}
			if n, err := strconv.ParseFloat(string(inner[start:i]), 64); err == nil {
				gsAdvance -= n
			}
		}
	}
	return gsAdvance
}

// parsePDFString parses a PDF string literal (literal or hex form).
func parsePDFString(s []byte) ([]byte, bool) {
	s = bytes.TrimSpace(s)
	if len(s) == 0 {
		return nil, false
	}
	if s[0] == '(' && s[len(s)-1] == ')' {
		return unescapePDFLiteralString(s[1 : len(s)-1]), true
	}
	if s[0] == '<' && s[len(s)-1] == '>' {
		inner := bytes.ReplaceAll(s[1:len(s)-1], []byte{' '}, []byte{})
		inner = bytes.ReplaceAll(inner, []byte{'\n'}, []byte{})
		inner = bytes.ReplaceAll(inner, []byte{'\r'}, []byte{})
		if len(inner)%2 != 0 {
			inner = append(inner, '0')
		}
		b := make([]byte, len(inner)/2)
		if _, err := hex.Decode(b, inner); err != nil {
			return nil, false
		}
		return b, true
	}
	return nil, false
}

// unescapePDFLiteralString unescapes a PDF literal string body (without outer parens).
func unescapePDFLiteralString(s []byte) []byte {
	var buf bytes.Buffer
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case '(', ')', '\\':
				buf.WriteByte(s[i])
			default:
				if s[i] >= '0' && s[i] <= '7' {
					octal := string(s[i])
					if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
						i++
						octal += string(s[i])
						if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
							i++
							octal += string(s[i])
						}
					}
					val, _ := strconv.ParseInt(octal, 8, 16)
					buf.WriteByte(byte(val))
				} else {
					buf.WriteByte(s[i])
				}
			}
		} else {
			buf.WriteByte(s[i])
		}
		i++
	}
	return buf.Bytes()
}

// decodeLatin1 converts bytes to UTF-8 using Latin-1, filtering controls.
func decodeLatin1(b []byte) []byte {
	var sb bytes.Buffer
	for _, c := range b {
		r := rune(c)
		if r >= 0x20 && r != 0x7f {
			sb.WriteRune(r)
		}
	}
	return sb.Bytes()
}

// ---------------------------------------------------------------------------
// ToUnicode CMap parser
// ---------------------------------------------------------------------------

func parseToUnicodeCMap(content []byte) map[uint16]string {
	m := make(map[uint16]string)
	text := string(content)

	for {
		start := strings.Index(text, "beginbfchar")
		if start < 0 {
			break
		}
		end := strings.Index(text[start:], "endbfchar")
		if end < 0 {
			break
		}
		parseBfChar(text[start+len("beginbfchar"):start+end], m)
		text = text[start+end+len("endbfchar"):]
	}

	text = string(content)
	for {
		start := strings.Index(text, "beginbfrange")
		if start < 0 {
			break
		}
		end := strings.Index(text[start:], "endbfrange")
		if end < 0 {
			break
		}
		parseBfRange(text[start+len("beginbfrange"):start+end], m)
		text = text[start+end+len("endbfrange"):]
	}

	return m
}

// scanHexTokens extracts all <...> hex tokens from s in order.
// It is whitespace-agnostic: tokens may be adjacent (<0000><0020>) or
// separated by spaces, as both forms appear in real PDF CMap streams.
func scanHexTokens(s string) []string {
	var tokens []string
	for {
		start := strings.Index(s, "<")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], ">")
		if end < 0 {
			break
		}
		tokens = append(tokens, s[start:start+end+1])
		s = s[start+end+1:]
	}
	return tokens
}

func parseBfChar(block string, m map[uint16]string) {
	// Each bfchar entry is exactly two hex tokens: <src> <dst>.
	// Tokens may be whitespace-separated or directly adjacent.
	// We therefore scan for <...> tokens per line rather than splitting on spaces.
	for line := range strings.SplitSeq(block, "\n") {
		toks := scanHexTokens(line)
		if len(toks) < 2 {
			continue
		}
		src, err := parseHexToken(toks[0])
		if err != nil {
			continue
		}
		dst, err := parseUnicodeHexToken(toks[1])
		if err != nil {
			continue
		}
		m[src] = dst
	}
}

func parseBfRange(block string, m map[uint16]string) {
	// Each bfrange entry is three tokens: <lo> <hi> <base-or-[array]>.
	// As with bfchar, tokens may appear without whitespace between them.
	for line := range strings.SplitSeq(block, "\n") {
		// Array form starts with '[' after the two code tokens; handle it
		// by checking the raw line before token extraction.
		if strings.Contains(line, "[") {
			continue // array form not yet supported
		}
		toks := scanHexTokens(line)
		if len(toks) < 3 {
			continue
		}
		lo, err := parseHexToken(toks[0])
		if err != nil {
			continue
		}
		hi, err := parseHexToken(toks[1])
		if err != nil {
			continue
		}
		base, err := parseUnicodeHexToken(toks[2])
		if err != nil {
			continue
		}
		baseRunes := []rune(base)
		if len(baseRunes) == 0 {
			continue
		}
		baseRune := baseRunes[len(baseRunes)-1]
		for code := lo; code <= hi; code++ {
			m[code] = string(baseRune + rune(code-lo))
		}
	}
}

func parseHexToken(s string) (uint16, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">"))
	val, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

func parseUnicodeHexToken(s string) (string, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">"))
	b, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", nil
	}
	if len(b)%2 == 0 && len(b) >= 2 {
		u16 := make([]uint16, len(b)/2)
		for i := 0; i < len(b); i += 2 {
			u16[i/2] = (uint16(b[i]) << 8) | uint16(b[i+1])
		}
		return string(utf16.Decode(u16)), nil
	}
	return string(rune(b[0])), nil
}

// ---------------------------------------------------------------------------
// Tokenizer
// ---------------------------------------------------------------------------

func tokenize(content []byte) [][]byte {
	var tokens [][]byte
	i, n := 0, len(content)

	for i < n {
		for i < n && isWhitespaceByte(content[i]) {
			i++
		}
		if i >= n {
			break
		}

		switch content[i] {
		case '%':
			for i < n && content[i] != '\n' && content[i] != '\r' {
				i++
			}

		case '(':
			start := i
			depth := 0
			i++
			for i < n {
				if content[i] == '\\' {
					i += 2
					continue
				}
				if content[i] == '(' {
					depth++
				} else if content[i] == ')' {
					if depth == 0 {
						i++
						break
					}
					depth--
				}
				i++
			}
			tokens = append(tokens, content[start:i])

		case '<':
			if i+1 < n && content[i+1] == '<' {
				// Dict — collect until the matching >>, skipping over nested
				// hex strings (<...>) so that <</Lang<6465>>> is handled
				// correctly and does not cause the hex-string branch to stall.
				start := i
				i += 2
				depth := 1
				for i < n && depth > 0 {
					switch content[i] {
					case '<':
						if i+1 < n && content[i+1] == '<' {
							depth++
							i += 2
						} else {
							i++
							for i < n && content[i] != '>' {
								i++
							}
							if i < n {
								i++
							}
						}
					case '>':
						if i+1 < n && content[i+1] == '>' {
							depth--
							i += 2
						} else {
							i++
						}
					case '(':
						i++
						pdepth := 0
						for i < n {
							if content[i] == '\\' {
								i += 2
								continue
							}
							if content[i] == '(' {
								pdepth++
							} else if content[i] == ')' {
								if pdepth == 0 {
									i++
									break
								}
								pdepth--
							}
							i++
						}
					default:
						i++
					}
				}
				tokens = append(tokens, content[start:i])
			} else {
				start := i
				i++
				for i < n && content[i] != '>' {
					i++
				}
				if i < n {
					i++
				}
				tokens = append(tokens, content[start:i])
			}

		case '[':
			start := i
			depth := 0
			i++
			for i < n {
				if content[i] == '(' {
					i++
					id := 0
					for i < n {
						if content[i] == '\\' {
							i += 2
							continue
						}
						if content[i] == '(' {
							id++
						} else if content[i] == ')' {
							if id == 0 {
								i++
								break
							}
							id--
						}
						i++
					}
					continue
				}
				if content[i] == '[' {
					depth++
				} else if content[i] == ']' {
					if depth == 0 {
						i++
						break
					}
					depth--
				}
				i++
			}
			tokens = append(tokens, content[start:i])

		case '/':
			start := i
			i++
			for i < n && !isWhitespaceByte(content[i]) && !isDelimiter(content[i]) {
				i++
			}
			tokens = append(tokens, content[start:i])

		default:
			start := i
			for i < n && !isWhitespaceByte(content[i]) && !isDelimiter(content[i]) {
				i++
			}
			if i > start {
				tokens = append(tokens, content[start:i])
			}
		}
	}

	return tokens
}

func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == 0
}

func isDelimiter(b byte) bool {
	return b == '(' || b == ')' || b == '<' || b == '>' || b == '[' || b == ']' ||
		b == '{' || b == '}' || b == '/' || b == '%'
}

func stripSlash(s []byte) []byte { return bytes.TrimPrefix(s, []byte{'/'}) }

func findClosingParen(s []byte, start int) int {
	depth, i := 0, start
	for i < len(s) {
		if s[i] == '\\' {
			i += 2
			continue
		}
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return -1
}

// ---------------------------------------------------------------------------
// Post-processing
// ---------------------------------------------------------------------------


func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '\u00C0' && r <= '\u024F')
}

// normaliseWhitespace collapses space runs, collapses newline runs, drops
// spaces before newlines, and trims leading/trailing whitespace.
func normaliseWhitespace(s []byte) bytes.Buffer {
	var sb bytes.Buffer
	prevNewline := false
	pendingSpace := false

	for _, r := range bytes.Runes(s) {
		switch r {
		case '\n', '\r':
			pendingSpace = false
			if !prevNewline {
				sb.WriteRune('\n')
			}
			prevNewline = true
		case ' ', '\t':
			if !prevNewline {
				pendingSpace = true
			}
		default:
			if pendingSpace {
				sb.WriteRune(' ')
				pendingSpace = false
			}
			sb.WriteRune(r)
			prevNewline = false
		}
	}
	return *bytes.NewBuffer(bytes.TrimSpace(sb.Bytes()))
}

// ---------------------------------------------------------------------------
// Standard encoding tables
// ---------------------------------------------------------------------------

func winAnsiEncoding() map[byte]rune {
	m := make(map[byte]rune, 256)
	for i := 0x20; i < 0x7F; i++ {
		m[byte(i)] = rune(i)
	}
	extras := map[byte]rune{
		0x80: '\u20AC', 0x82: '\u201A', 0x83: '\u0192', 0x84: '\u201E',
		0x85: '\u2026', 0x86: '\u2020', 0x87: '\u2021', 0x88: '\u02C6',
		0x89: '\u2030', 0x8A: '\u0160', 0x8B: '\u2039', 0x8C: '\u0152',
		0x8E: '\u017D', 0x91: '\u2018', 0x92: '\u2019', 0x93: '\u201C',
		0x94: '\u201D', 0x95: '\u2022', 0x96: '\u2013', 0x97: '\u2014',
		0x98: '\u02DC', 0x99: '\u2122', 0x9A: '\u0161', 0x9B: '\u203A',
		0x9C: '\u0153', 0x9E: '\u017E', 0x9F: '\u0178',
	}
	for i := 0xA0; i < 0x100; i++ {
		m[byte(i)] = rune(i)
	}
	maps.Copy(m, extras)
	return m
}

func macRomanEncoding() map[byte]rune {
	m := make(map[byte]rune, 256)
	for i := 0x20; i < 0x7F; i++ {
		m[byte(i)] = rune(i)
	}
	macHigh := []rune{
		'\u00C4', '\u00C5', '\u00C7', '\u00C9', '\u00D1', '\u00D6', '\u00DC', '\u00E1',
		'\u00E0', '\u00E2', '\u00E4', '\u00E5', '\u00E7', '\u00E9', '\u00E8', '\u00EA',
		'\u00EB', '\u00ED', '\u00EC', '\u00EE', '\u00EF', '\u00F1', '\u00F3', '\u00F2',
		'\u00F4', '\u00F6', '\u00FA', '\u00F9', '\u00FB', '\u00FC', '\u2020', '\u00B0',
		'\u00A2', '\u00A3', '\u00A7', '\u2022', '\u00B6', '\u00DF', '\u00AE', '\u00A9',
		'\u2122', '\u00B4', '\u00A8', '\u2260', '\u00C6', '\u00D8', '\u221E', '\u00B1',
		'\u2264', '\u2265', '\u00A5', '\u00B5', '\u2202', '\u2211', '\u220F', '\u03C0',
		'\u222B', '\u00AA', '\u00BA', '\u03A9', '\u00E6', '\u00F8', '\u00BF', '\u00A1',
		'\u00AC', '\u221A', '\u0192', '\u2248', '\u2206', '\u00AB', '\u00BB', '\u2026',
		'\u00A0', '\u00C0', '\u00C3', '\u00D5', '\u0152', '\u0153', '\u2013', '\u2014',
		'\u201C', '\u201D', '\u2018', '\u2019', '\u00F7', '\u25CA', '\u00FF', '\u0178',
		'\u2044', '\u20AC', '\u2039', '\u203A', '\uFB01', '\uFB02', '\u2021', '\u00B7',
		'\u201A', '\u201E', '\u2030', '\u00C2', '\u00CA', '\u00C1', '\u00CB', '\u00C8',
		'\u00CD', '\u00CE', '\u00CF', '\u00CC', '\u00D3', '\u00D4', '\uF8FF', '\u00D2',
		'\u00DA', '\u00DB', '\u00D9', '\u0131', '\u02C6', '\u02DC', '\u00AF', '\u02D8',
		'\u02D9', '\u02DA', '\u00B8', '\u02DD', '\u02DB', '\u02C7',
	}
	for i, r := range macHigh {
		m[byte(0x80+i)] = r
	}
	return m
}

func glyphToRune(name string) (rune, bool) {
	if r, ok := glyphNames[name]; ok {
		return r, true
	}
	if len(name) == 1 {
		return rune(name[0]), true
	}
	if strings.HasPrefix(name, "uni") {
		if val, err := strconv.ParseInt(name[3:], 16, 32); err == nil {
			return rune(val), true
		}
	}
	return 0, false
}
