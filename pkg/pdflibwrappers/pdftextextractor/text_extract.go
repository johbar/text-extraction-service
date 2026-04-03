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

// ExtractPageText extracts the readable text from the given page of ctx.
// pageNr is 1-based. Returns a PageText with the decoded text or an error.
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

	// Retrieve raw content stream bytes for this page.
	content, err := ctx.XRefTable.PageContent(pageDict, pageNr)
	if err != nil {
		if err == model.ErrNoContent {
			return nil, nil
		}
		return nil, fmt.Errorf("extractPageText: page %d content: %w", pageNr, err)
	}

	// Build a font map from the inherited resource dict so we can decode text.
	fontMap := buildFontMap(ctx.XRefTable, inhPAttrs.Resources)

	text, err := extractTextFromContent(content, fontMap)
	if err != nil {
		return nil, fmt.Errorf("extractPageText: page %d parse: %w", pageNr, err)
	}

	return &text, nil
}

// ---------------------------------------------------------------------------
// Font encoding helpers
// ---------------------------------------------------------------------------

// pdfFont represents just enough font metadata to decode text operands.
type pdfFont struct {
	// encoding maps a single-byte character code to a Unicode rune.
	// nil means use the identity / Latin-1 mapping as a fallback.
	encoding map[byte]rune
	// toUnicode maps a character code (as a 2-byte big-endian uint16) to a
	// Unicode string. When present this takes precedence over encoding.
	toUnicode map[uint16]string
}

// decodeBytes converts a slice of raw character code bytes using the font's
// encoding tables and returns the corresponding Unicode string.
func (f *pdfFont) decodeBytes(b []byte) []byte {
	var sb bytes.Buffer
	for i := 0; i < len(b); {
		// Try 2-byte lookup first when toUnicode is available.
		if f.toUnicode != nil && i+1 < len(b) {
			code := (uint16(b[i]) << 8) | uint16(b[i+1])
			if s, ok := f.toUnicode[code]; ok {
				sb.WriteString(s)
				i += 2
				continue
			}
		}
		if f.toUnicode != nil {
			code := uint16(b[i])
			if s, ok := f.toUnicode[code]; ok {
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
		// Fallback: treat as Latin-1 but skip control characters.
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
		f := &pdfFont{}

		// Try to parse a ToUnicode CMap.
		if tuRef, ok := fd.Find("ToUnicode"); ok {
			if sd, _, err := xRefTable.DereferenceStreamDict(tuRef); err == nil && sd != nil {
				if err := sd.Decode(); err == nil {
					f.toUnicode = parseToUnicodeCMap(sd.Content)
				}
			}
		}

		// Parse the Encoding entry if present.
		if encObj, ok := fd.Find("Encoding"); ok {
			f.encoding = parseEncoding(xRefTable, encObj)
		}

		fontMap[name] = f
	}

	return fontMap
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
		// Start from a base encoding if specified.
		var base map[byte]rune
		if baseObj, ok := v.Find("BaseEncoding"); ok {
			if n, ok := baseObj.(types.Name); ok {
				base = namedEncoding(n.Value())
			}
		}
		if base == nil {
			base = maps.Clone(standardEnc)
		}

		// Apply Differences array.
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

// namedEncoding returns a copy of one of the standard PDF named encodings.
func namedEncoding(name string) map[byte]rune {

	switch name {
	case "MacRomanEncoding":
		return maps.Clone(macRomanEnc)
	case "WinAnsiEncoding":
		return maps.Clone(winAnsiEnc)
	default: // StandardEncoding and fallback
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

	// Text line matrix origin — the absolute position of the start of the
	// current text line. Updated by Tm (absolute set) and Td/TD (relative
	// offset). T* uses the stored leading to compute a relative offset.
	// tlTx/tlTy are in unscaled user-space points.
	tlTx, tlTy float64
	tlSet      bool // true once we have a reference position

	// leading is the current text leading (set by TL, or implicitly by TD).
	leading float64

	// Current font size from the most recent Tf operator.
	fontSize float64
}

// emitBreakDelta writes a newline or space to w based on the per-operator
// position delta (dx, dy). dy is positive when moving up the page (PDF
// coordinate system), negative when moving down.
func (ts *textState) emitBreakDelta(w io.ByteWriter, dx, dy float64) {
	lineThreshold := ts.fontSize * 0.5
	spaceThreshold := ts.fontSize * 0.1
	if lineThreshold < 1 {
		lineThreshold = 1
	}
	if dy > lineThreshold || dy < -lineThreshold {
		w.WriteByte('\n')
	} else if dx*(ts.wordSpacing) > spaceThreshold {
		// FIXME: This is mostly redundant, sometimes misplaced
		w.WriteByte(' ')
	}
	// Small/zero/negative dx with no vertical movement: contiguous chunk,
	// emit nothing.
}

// extractTextFromContent parses a PDF content stream and returns the plain text.
// It handles the most common text-showing operators:
//
//	Tj  – show string
//	TJ  – show array of strings/numbers
//	'   – move to next line and show string
//	"   – set spacing, move to next line, show string
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
			ts.tlSet = false // reset position tracking for each text block
		case "ET":
			if ts.inBT {
				out.WriteRune('\n')
			}
			ts.inBT = false
			ts.tlSet = false

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
			// leading TL — sets the text leading used by T*.
			if i >= 1 {
				ts.leading, _ = strconv.ParseFloat(string(tokens[i-1]), 64)
			}

		case "Tw":
			if i >= 1 {
				ts.wordSpacing, _ = strconv.ParseFloat(string(tokens[i-1]), 64)
			}

		case "Td", "TD":
			// tx ty Td — move the text line position by (tx, ty) relative to
			// the start of the current line. ty == 0 means a purely horizontal
			// shift (same line); any non-zero ty is a vertical move.
			// TD additionally sets the leading to -ty.
			if ts.inBT && i >= 2 {
				dy, errY := strconv.ParseFloat(string(tokens[i-1]), 64)
				dx, errX := strconv.ParseFloat(string(tokens[i-2]), 64)
				if errY == nil && errX == nil {
					if bytes.Equal(tok, []byte{'T', 'D'}) {
						ts.leading = -dy
					}
					ts.tlTx += dx
					ts.tlTy += dy
					if !ts.tlSet {
						ts.tlSet = true
					} else {
						ts.emitBreakDelta(&out, dx, dy)
					}
				}
			}

		case "T*":
			// Move to the start of the next line, equivalent to 0 -leading Td.
			if ts.inBT {
				dy := -ts.leading
				ts.tlTy += dy
				ts.tlTx = 0
				if !ts.tlSet {
					ts.tlSet = true
				} else {
					ts.emitBreakDelta(&out, 0, dy)
				}
			}

		case "Tm":
			// a b c d tx ty Tm — sets the text matrix and text line matrix
			// to an absolute position. We compare against the previous line
			// origin to decide between a new line and a same-line reposition.
			if ts.inBT && i >= 6 {
				newTy, errY := strconv.ParseFloat(string(tokens[i-1]), 64)
				newTx, errX := strconv.ParseFloat(string(tokens[i-2]), 64)
				if errY == nil && errX == nil {
					if !ts.tlSet {
						// First position in this BT block — nothing to compare.
						ts.tlTx, ts.tlTy = newTx, newTy
						ts.tlSet = true
					} else {
						prevTx, prevTy := ts.tlTx, ts.tlTy
						ts.tlTx, ts.tlTy = newTx, newTy
						// dy is positive when the new position is above the old
						// one (PDF y-axis points up), matching Td/TD convention.
						ts.emitBreakDelta(&out, newTx-prevTx, prevTy-newTy)
					}
				}
			}

		case "Tj":
			// string Tj
			if ts.inBT && i >= 1 {
				text := decodeOperand(tokens[i-1], ts.currentFont)
				out.Write(text)
			}

		case "'":
			// string '  — newline then show string
			if ts.inBT && i >= 1 {
				out.WriteRune('\n')
				text := decodeOperand(tokens[i-1], ts.currentFont)
				out.Write(text)
			}

		case "\"":
			// aw ac string "  — set spacing, newline, show string
			if ts.inBT && i >= 1 {
				out.WriteRune('\n')
				text := decodeOperand(tokens[i-1], ts.currentFont)
				out.Write(text)
			}

		case "TJ":
			// array TJ
			if ts.inBT && i >= 1 {
				text := decodeTJArray(tokens[i-1], ts.currentFont)
				out.Write(text)
			}
		}

		i++
	}

	// return dehyphenate(normaliseWhitespace(out.String())), nil
	return normaliseWhitespace(out.Bytes()), nil

}

// decodeOperand decodes a PDF string literal or hex string operand.
// It handles both ( literal ) and < hex > forms.
func decodeOperand(tok []byte, f *pdfFont) []byte {
	raw, ok := parsePDFString(tok)
	if !ok {
		return []byte{}
	}
	if f == nil {
		return decodeLatin1(raw)
	}
	return f.decodeBytes(raw)
}

// decodeTJArray decodes the array operand of a TJ operator.
// Elements are alternating string operands and numeric kerning offsets.
func decodeTJArray(tok []byte, f *pdfFont) []byte {
	tok = bytes.TrimSpace(tok)
	if tok[0] != '[' || tok[len(tok)-1] != ']' {
		return []byte{}
	}
	inner := tok[1 : len(tok)-1]
	var sb bytes.Buffer
	i := 0
	for i < len(inner) {
		// Skip whitespace.
		for i < len(inner) && isWhitespace(inner[i]) {
			i++
		}
		if i >= len(inner) {
			break
		}

		if inner[i] == '(' {
			// Find matching unescaped ')'.
			end := findClosingParen(inner, i)
			if end < 0 {
				break
			}
			segment := inner[i : end+1]
			sb.Write(decodeOperand(segment, f))
			i = end + 1
		} else if inner[i] == '<' {
			end := bytes.Index(inner[i:], []byte{'>'})
			if end < 0 {
				break
			}
			segment := inner[i : i+end+1]
			sb.Write(decodeOperand(segment, f))
			i += end + 1
		} else {
			// Numeric kerning offset — skip until whitespace or next string.
			for i < len(inner) && !isWhitespace(inner[i]) && inner[i] != '(' && inner[i] != '<' {
				i++
			}
		}
	}
	return sb.Bytes()
}

// parsePDFString parses a PDF string literal:
//   - ( ... )  — literal string (may contain escape sequences)
//   - < ... >  — hex string
//
// Returns the raw byte slice and true on success.
func parsePDFString(s []byte) ([]byte, bool) {
	s = bytes.TrimSpace(s)
	if len(s) == 0 {
		return nil, false
	}

	if s[0] == '(' && s[len(s)-1] == ')' {
		// Literal string – unescape.
		inner := s[1 : len(s)-1]
		return unescapePDFLiteralString(inner), true
	}

	if s[0] == '<' && s[len(s)-1] == '>' {
		// Hex string.
		inner := bytes.ReplaceAll(s[1:len(s)-1], []byte{' '}, []byte{})
		inner = bytes.ReplaceAll(inner, []byte{'\n'}, []byte{})
		inner = bytes.ReplaceAll(inner, []byte{'\r'}, []byte{})
		if len(inner)%2 != 0 {
			inner = append(inner, '0')
		}
		b := make([]byte, len(inner)/2)
		_, err := hex.Decode(b, inner)
		if err != nil {
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
			case '(':
				buf.WriteByte('(')
			case ')':
				buf.WriteByte(')')
			case '\\':
				buf.WriteByte('\\')
			default:
				// Octal escape \ddd
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

// decodeLatin1 converts a byte slice to a string using Latin-1/ISO-8859-1,
// filtering out non-printable characters.
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

// parseToUnicodeCMap extracts the bfchar and bfrange mappings from a ToUnicode CMap stream.
func parseToUnicodeCMap(content []byte) map[uint16]string {
	m := make(map[uint16]string)
	text := string(content)

	// beginbfchar / endbfchar blocks.
	for {
		start := strings.Index(text, "beginbfchar")
		if start < 0 {
			break
		}
		end := strings.Index(text[start:], "endbfchar")
		if end < 0 {
			break
		}
		block := text[start+len("beginbfchar") : start+end]
		parseBfChar(block, m)
		text = text[start+end+len("endbfchar"):]
	}

	// Reset and parse bfrange.
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
		block := text[start+len("beginbfrange") : start+end]
		parseBfRange(block, m)
		text = text[start+end+len("endbfrange"):]
	}

	return m
}

func parseBfChar(block string, m map[uint16]string) {
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		src, err := parseHexToken(parts[0])
		if err != nil {
			continue
		}
		dst, err := parseUnicodeHexToken(parts[1])
		if err != nil {
			continue
		}
		m[src] = dst
	}
}

func parseBfRange(block string, m map[uint16]string) {
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		lo, err := parseHexToken(parts[0])
		if err != nil {
			continue
		}
		hi, err := parseHexToken(parts[1])
		if err != nil {
			continue
		}

		// Third element may be a hex token (single mapping) or an array.
		third := parts[2]
		if strings.HasPrefix(third, "[") {
			// Array form – not yet supported; fall through.
			continue
		}

		base, err := parseUnicodeHexToken(third)
		if err != nil {
			continue
		}
		// base as rune value.
		baseRunes := []rune(base)
		if len(baseRunes) == 0 {
			continue
		}
		baseRune := baseRunes[len(baseRunes)-1]

		for code := lo; code <= hi; code++ {
			delta := rune(code - lo)
			newRune := baseRune + delta
			m[code] = string(newRune)
		}
	}
}

// parseHexToken parses a <XXXX> token into a uint16 character code.
func parseHexToken(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	val, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

// parseUnicodeHexToken parses a <XXXX> hex token into a Unicode string.
// The hex bytes are interpreted as UTF-16BE if length >= 4, else as a code point.
func parseUnicodeHexToken(s string) (string, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")

	b, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}

	if len(b) == 0 {
		return "", nil
	}

	if len(b)%2 == 0 && len(b) >= 2 {
		// Interpret as UTF-16BE.
		u16 := make([]uint16, len(b)/2)
		for i := 0; i < len(b); i += 2 {
			u16[i/2] = (uint16(b[i]) << 8) | uint16(b[i+1])
		}
		return string(utf16.Decode(u16)), nil
	}
	// Single byte.
	return string(rune(b[0])), nil
}

// ---------------------------------------------------------------------------
// Tokenizer
// ---------------------------------------------------------------------------

// tokenize splits a PDF content stream into tokens: operators, names,
// numbers, string literals, and array literals.
// This is a simplified tokenizer sufficient for text extraction.
func tokenize(content []byte) [][]byte {
	var tokens [][]byte
	i := 0
	n := len(content)

	for i < n {
		// Skip whitespace.
		for i < n && isWhitespaceByte(content[i]) {
			i++
		}
		if i >= n {
			break
		}

		switch content[i] {
		case '%':
			// Comment – skip to end of line.
			for i < n && content[i] != '\n' && content[i] != '\r' {
				i++
			}

		case '(':
			// Literal string – collect until matching unescaped ')'.
			start := i
			depth := 0
			i++ // skip opening '('
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
			tokens = append(tokens, (content[start:i]))

		case '<':
			if i+1 < n && content[i+1] == '<' {
				// Dict – collect until >>
				start := i
				i += 2
				for i < n-1 {
					if content[i] == '>' && content[i+1] == '>' {
						i += 2
						break
					}
					i++
				}
				tokens = append(tokens, (content[start:i]))
			} else {
				// Hex string.
				start := i
				i++
				for i < n && content[i] != '>' {
					i++
				}
				if i < n {
					i++
				}
				tokens = append(tokens, (content[start:i]))
			}

		case '[':
			// Array – collect until matching ']'.
			start := i
			depth := 0
			i++
			for i < n {
				if content[i] == '(' {
					// String inside array.
					i++
					innerDepth := 0
					for i < n {
						if content[i] == '\\' {
							i += 2
							continue
						}
						if content[i] == '(' {
							innerDepth++
						} else if content[i] == ')' {
							if innerDepth == 0 {
								i++
								break
							}
							innerDepth--
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
			tokens = append(tokens, (content[start:i]))

		case '/':
			// Name object.
			start := i
			i++
			for i < n && !isWhitespaceByte(content[i]) && !isDelimiter(content[i]) {
				i++
			}
			tokens = append(tokens, (content[start:i]))

		default:
			// Number or operator.
			start := i
			for i < n && !isWhitespaceByte(content[i]) && !isDelimiter(content[i]) {
				i++
			}
			if i > start {
				tokens = append(tokens, (content[start:i]))
			}
		}
	}

	return tokens
}

func isWhitespaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f' || b == 0
}

func isWhitespace(b byte) bool {
	return isWhitespaceByte(b)
}

func isDelimiter(b byte) bool {
	return b == '(' || b == ')' || b == '<' || b == '>' || b == '[' || b == ']' ||
		b == '{' || b == '}' || b == '/' || b == '%'
}

func stripSlash(s []byte) []byte {
	return bytes.TrimPrefix(s, []byte{'/'})
}

// findClosingParen finds the index of the ')' that closes the '(' at position
// start within s, respecting escape sequences and nested parens.
func findClosingParen(s []byte, start int) int {
	depth := 0
	i := start
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

// dehyphenate joins words that were broken across lines with a hyphen.
// It handles two cases:
//
//  1. Soft hyphen (U+00AD) immediately before a newline — the hyphen is a
//     formatting artifact and is dropped entirely: "hyphen\u00ad\nated" → "hyphenated".
//
//  2. Hard hyphen ('-') at the end of a line where both the preceding and
//     following substrings look like word fragments (all letters). In that case
//     the hyphen and newline are dropped and the fragments joined:
//     "hyphen-\nated" → "hyphenated".
//     If either side contains digits or the preceding part ends with another
//     hyphen (e.g. "state-of-\nthe-art") the hyphen is retained and only the
//     newline is replaced with a space, preserving the intended punctuation.
func dehyphenate(s string) string {
	runes := []rune(s)
	var out []rune
	i := 0
	for i < len(runes) {
		r := runes[i]

		// Soft hyphen followed by newline → drop both.
		if r == '\u00AD' && i+1 < len(runes) && runes[i+1] == '\n' {
			i += 2 // skip soft-hyphen and newline
			continue
		}

		// Hard hyphen followed by newline → check context.
		if r == '-' && i+1 < len(runes) && runes[i+1] == '\n' {
			before := trailingWord(out)
			after := leadingWord(runes[i+2:])
			if isWordFragment(before) && isWordFragment(after) {
				// Looks like typographic line-breaking — drop hyphen and newline.
				i += 2
				continue
			}
			// Genuine hyphen (compound word, number range, etc.) — keep the
			// hyphen but replace the newline with a space so the line still joins.
			out = append(out, '-')
			out = append(out, ' ')
			i += 2
			continue
		}

		out = append(out, r)
		i++
	}
	return string(out)
}

// trailingWord returns the last uninterrupted run of letters from buf.
func trailingWord(buf []rune) []rune {
	end := len(buf)
	start := end
	for start > 0 && isLetter(buf[start-1]) {
		start--
	}
	return buf[start:end]
}

// leadingWord returns the first uninterrupted run of letters from s.
func leadingWord(s []rune) []rune {
	end := 0
	for end < len(s) && isLetter(s[end]) {
		end++
	}
	return s[:end]
}

// isWordFragment reports whether frag is non-empty and consists entirely of
// Unicode letters — the hallmark of a hyphenation fragment rather than a
// number range or compound term with digits.
func isWordFragment(frag []rune) bool {
	if len(frag) == 0 {
		return false
	}
	for _, r := range frag {
		if !isLetter(r) {
			return false
		}
	}
	return true
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '\u00C0' && r <= '\u024F') // Latin extended — covers accented chars
}

// normaliseWhitespace collapses runs of spaces into a single space, collapses
// runs of newlines into a single newline, drops spaces that immediately
// precede a newline, and trims leading/trailing whitespace.
func normaliseWhitespace(s []byte) bytes.Buffer {
	var sb bytes.Buffer
	prevNewline := false
	pendingSpace := false

	for _, r := range bytes.Runes(s) {
		switch r {
		case '\n', '\r':
			// Discard any pending space — it would only trail a line.
			pendingSpace = false
			if !prevNewline {
				sb.WriteRune('\n')
			}
			prevNewline = true
		case ' ', '\t':
			if !prevNewline {
				// Defer the space; we'll emit it only if the next character
				// is not a newline.
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
	return sb
	// return strings.TrimSpace(sb.String())
}

// ---------------------------------------------------------------------------
// Standard encoding tables
// ---------------------------------------------------------------------------

// standardEncoding returns the Adobe Standard Encoding as a map.

// winAnsiEncoding returns Windows-1252 encoding as a map.
func winAnsiEncoding() map[byte]rune {
	m := make(map[byte]rune, 256)
	// ASCII range.
	for i := 0x20; i < 0x7F; i++ {
		m[byte(i)] = rune(i)
	}
	// Windows-1252 extensions.
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
		m[byte(i)] = rune(i) // Latin-1 supplement.
	}
	maps.Copy(m, extras)
	return m
}

// macRomanEncoding returns the Mac Roman encoding.
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

// glyphToRune converts a PDF glyph name (e.g. "space", "A", "AE") to a rune.
func glyphToRune(name string) (rune, bool) {
	if r, ok := glyphNames[name]; ok {
		return r, true
	}
	// Single letter names map to themselves.
	if len(name) == 1 {
		return rune(name[0]), true
	}
	// uni<XXXX> names.
	if strings.HasPrefix(name, "uni") {
		val, err := strconv.ParseInt(name[3:], 16, 32)
		if err == nil {
			return rune(val), true
		}
	}
	return 0, false
}
