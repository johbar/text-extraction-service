/*
Package pdftextextractor implements a text extractor for PDFs ontop of pdfcpu.
It might not be on par with C++ PDF libs like PDFium or Poppler regarding accuracy and quality
but does a decent job and is pure Go.
*/
package pdftextextractor

import (
	"bytes"
	"encoding/hex"
	"maps"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/johbar/pdfcpu-lite/pkg/pdfcpu/model"
	"github.com/johbar/pdfcpu-lite/pkg/pdfcpu/types"
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
// encoding tables, writing the result directly into dst.
func (f *pdfFont) decodeBytes(b []byte, dst *bytes.Buffer) {
	for i := 0; i < len(b); {
		if f.toUnicode != nil && i+1 < len(b) {
			code := (uint16(b[i]) << 8) | uint16(b[i+1])
			if s, ok := f.toUnicode[code]; ok {
				dst.WriteString(s)
				i += 2
				continue
			}
		}
		if f.toUnicode != nil {
			if s, ok := f.toUnicode[uint16(b[i])]; ok {
				dst.WriteString(s)
				i++
				continue
			}
		}
		if f.encoding != nil {
			if r, ok := f.encoding[b[i]]; ok {
				dst.WriteRune(r)
				i++
				continue
			}
		}
		r := rune(b[i])
		if r >= 0x20 && r != 0x7f {
			dst.WriteRune(r)
		}
		i++
	}
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

// xObject holds the decoded content stream and resolved font map for a Form
// XObject.  The matrix field is the XObject's own /Matrix entry, which must be
// pre-multiplied into the caller's CTM before parsing the stream (PDF spec
// §8.10.1).  It is the identity when the XObject has no /Matrix entry.
type xObject struct {
	content []byte
	fontMap map[string]*pdfFont
	xobjMap map[string]xObject // nested XObjects from this XObject's /Resources
	matrix  matrix3
}

// buildXObjMap collects all Form XObjects from a page's resource dictionary.
// Only /Subtype /Form streams carry renderable text; image XObjects are skipped.
// Nested XObjects (Form XObjects that themselves reference other Form XObjects)
// are resolved recursively so that a Do operator inside a Form XObject works.
func buildXObjMap(xRefTable *model.XRefTable, resources types.Dict) map[string]xObject {
	return buildXObjMapDepth(xRefTable, resources, 0)
}

const maxXObjDepth = 8 // guard against pathological circular references

func buildXObjMapDepth(xRefTable *model.XRefTable, resources types.Dict, depth int) map[string]xObject {
	m := make(map[string]xObject)
	if resources == nil || depth > maxXObjDepth {
		return m
	}

	xobjRef, found := resources.Find("XObject")
	if !found {
		return m
	}
	xobjDict, err := xRefTable.DereferenceDict(xobjRef)
	if err != nil || xobjDict == nil {
		return m
	}

	for name, ref := range xobjDict {
		sd, _, err := xRefTable.DereferenceStreamDict(ref)
		if err != nil || sd == nil {
			continue
		}
		// Only process Form XObjects; skip Image and other subtypes.
		subtype, _ := sd.Dict.Find("Subtype")
		if n, ok := subtype.(types.Name); !ok || n.Value() != "Form" {
			continue
		}
		if err := sd.Decode(); err != nil {
			continue
		}

		// /Matrix: the XObject's coordinate transform (defaults to identity).
		mat := identityMatrix()
		if matObj, ok := sd.Dict.Find("Matrix"); ok {
			if matArr, err := xRefTable.DereferenceArray(matObj); err == nil && len(matArr) == 6 {
				vals := [6]float64{}
				allOK := true
				for i, v := range matArr {
					switch n := v.(type) {
					case types.Float:
						vals[i] = n.Value()
					case types.Integer:
						vals[i] = float64(n.Value())
					default:
						allOK = false
					}
				}
				if allOK {
					mat = matrix3{a: vals[0], b: vals[1], c: vals[2], d: vals[3], e: vals[4], f: vals[5]}
				}
			}
		}

		// /Resources: the XObject's own font and XObject dictionaries.
		// These are merged with the parent's — child fonts shadow parent fonts
		// when names collide, which is the correct scoping behaviour.
		var xobjResources types.Dict
		if resRef, ok := sd.Dict.Find("Resources"); ok {
			xobjResources, _ = xRefTable.DereferenceDict(resRef)
		}

		m[name] = xObject{
			content: sd.Content,
			fontMap: buildFontMap(xRefTable, xobjResources),
			xobjMap: buildXObjMapDepth(xRefTable, xobjResources, depth+1),
			matrix:  mat,
		}
	}
	return m
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
// 3×3 matrix arithmetic (column-major, PDF convention)
// ---------------------------------------------------------------------------

// matrix3 is a 3×3 transformation matrix stored in the six meaningful
// components of the PDF "a b c d e f" form. The bottom row is always
// [0 0 1] and is never stored explicitly.
//
// PDF column-vector convention (point on the right):
//
//	| a  b  0 |
//	| c  d  0 |
//	| e  f  1 |   ← this is how PDF spec lays it out for row vectors
//
// For our purposes we carry [a b c d e f] and compose as:
//
//	result = left × right
//	result.e = left.a*right.e + left.c*right.f + left.e
//	…etc.
type matrix3 struct {
	a, b, c, d, e, f float64
}

// identityMatrix returns the 3×3 identity.
func identityMatrix() matrix3 {
	return matrix3{a: 1, d: 1}
}

// multiply returns m × n (PDF row-vector convention).
func (m matrix3) multiply(n matrix3) matrix3 {
	return matrix3{
		a: m.a*n.a + m.b*n.c,
		b: m.a*n.b + m.b*n.d,
		c: m.c*n.a + m.d*n.c,
		d: m.c*n.b + m.d*n.d,
		e: m.e*n.a + m.f*n.c + n.e,
		f: m.e*n.b + m.f*n.d + n.f,
	}
}

// transformPoint maps text-space point (x, y) through m into device space.
func (m matrix3) transformPoint(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// scaleX returns the x-axis scale factor of the matrix (length of the x
// basis vector), used to derive the effective font size in device space.
func (m matrix3) scaleX() float64 {
	return math.Sqrt(m.a*m.a + m.b*m.b)
}

// ---------------------------------------------------------------------------
// Content stream parser
// ---------------------------------------------------------------------------

// graphicsState holds the subset of the PDF graphics state needed for text
// extraction: the current transformation matrix and its save/restore stack.
type graphicsState struct {
	ctm   matrix3
	stack []matrix3
}

func newGraphicsState() graphicsState {
	return graphicsState{ctm: identityMatrix()}
}

// push saves a copy of the current CTM onto the stack (PDF operator q).
func (gs *graphicsState) push() {
	gs.stack = append(gs.stack, gs.ctm)
}

// pop restores the most recently saved CTM (PDF operator Q).
// A Q without a matching q is silently ignored, as in most PDF viewers.
func (gs *graphicsState) pop() {
	if n := len(gs.stack); n > 0 {
		gs.ctm = gs.stack[n-1]
		gs.stack = gs.stack[:n-1]
	}
}

// textState tracks the PDF text state machine during content stream parsing.
// All position arithmetic is performed in device space (after composing the
// current transformation matrix with the text matrix and the text line matrix).
type textState struct {
	inBT        bool
	currentFont *pdfFont
	fontMap     map[string]*pdfFont

	// charSpacing is the Tc text state parameter (set by the "Tc" operator and
	// by the " operator). It is added to the advance of every glyph in text
	// space units. PDF spec §9.3.2.
	charSpacing float64

	// wordSpacing is the Tw text state parameter (set by the "Tw" operator and
	// by the " operator). It is added to the advance of every glyph whose
	// character code is 0x20 (the single-byte word-space code) in text space
	// units. PDF spec §9.3.3.
	wordSpacing float64

	// Text line matrix (Tlm) and text matrix (Tm) per PDF spec §9.4.
	// Tlm is the reference point updated by Td/TD/T*/Tm.
	// After every showing operator Tm is also updated (Tlm is not).
	// We store both as full 3×3 matrices so that composition with the CTM
	// is exact regardless of rotation, shear, or non-uniform scale.
	tlm matrix3 // text line matrix
	tm  matrix3 // text matrix (equals tlm at line-start; advances with glyphs)

	// tlSet is true once at least one positioning operator has fired inside
	// the current BT/ET block, making tlm/tm valid reference points.
	tlSet bool

	// cursorDevX/cursorDevY is the device-space pen position after the last
	// rendered glyph. It is compared against the device-space origin of the
	// next text chunk to decide whether to emit a space or newline.
	cursorDevX, cursorDevY float64

	// leading is the current text leading (set by TL, implied by TD).
	leading float64

	// tfSize is the raw size argument from the most recent Tf operator.
	tfSize float64

	// fontSize is the effective rendered size in device-space units.
	// Computed as tfSize × ||CTM × Tm||(x-scale) whenever Tm or the CTM
	// changes. Used for all gap-detection thresholds.
	fontSize float64

	currentMCID int // Tracks the active logical block ID (-1 if none)
	th          float64
}

// deviceOrigin maps the current text-line-matrix origin through the CTM
// to obtain the device-space coordinates of the start of the current line.
func (ts *textState) deviceOrigin(gs *graphicsState) (float64, float64) {
	combined := ts.tlm.multiply(gs.ctm)
	return combined.transformPoint(0, 0)
}

// updateFontSize recomputes the effective device-space font size from tfSize
// and the x-scale of the combined Tm × CTM matrix.
func (ts *textState) updateFontSize(gs *graphicsState) {
	if ts.tfSize == 0 {
		ts.fontSize = 0
		return
	}
	combined := ts.tm.multiply(gs.ctm)
	scale := combined.scaleX()
	if scale == 0 {
		scale = 1
	}
	ts.fontSize = ts.tfSize * scale
}

// setTm replaces both the text matrix and the text line matrix with mat,
// recomputes the effective font size, and marks the state as positioned.
func (ts *textState) setTm(mat matrix3, gs *graphicsState) {
	ts.tlm = mat
	ts.tm = mat
	ts.updateFontSize(gs)
	ts.tlSet = true
}

// applyTd moves the text line matrix by the text-space offset (tx, ty)
// (a translation appended to the current line matrix) and resets the text
// matrix to the new line matrix.
func (ts *textState) applyTd(tx, ty float64, gs *graphicsState) {
	// Build a pure-translation matrix and concatenate on the right of tlm.
	// This matches the PDF spec: Tlm′ = [1 0 0 1 tx ty] × Tlm
	trans := matrix3{a: 1, d: 1, e: tx, f: ty}
	ts.tlm = trans.multiply(ts.tlm)
	ts.tm = ts.tlm
	ts.updateFontSize(gs)
	ts.tlSet = true
}

// advanceTm moves the text matrix (not the line matrix) forward by the full
// text-space advance of the glyph sequence encoded in raw bytes b, following
// PDF spec §9.4.4:
//
//	tx = (w₀/1000 + Tc) × Tfs          for every glyph
//	tx += Tw × Tfs                       additionally for code 0x20 (word space)
//
// Both Tc (charSpacing) and Tw (wordSpacing) are applied in text space so that
// the cursor lands exactly where the PDF renderer would place the next glyph,
// preventing false inter-word spaces from being inserted by emitGap.
func (ts *textState) advanceTm(b []byte, gs *graphicsState) {
	if ts.tfSize == 0 {
		return
	}
	tx := ts.rawBytesAdvance(b)
	adv := matrix3{a: 1, d: 1, e: tx}
	ts.tm = adv.multiply(ts.tm)
	combined := ts.tm.multiply(gs.ctm)
	ts.cursorDevX, ts.cursorDevY = combined.transformPoint(0, 0)
}

// advanceTmGS is like advanceTm but accepts a pre-computed net glyph-space
// advance (kerning already folded in) plus the original raw bytes, so that Tc
// and Tw can be applied per character. Used by TJ which interleaves strings
// with kerning numbers: the kerning part is passed in gsKernAdj (already
// accumulated as a signed glyph-space delta), while Tc/Tw are derived from b.
func (ts *textState) advanceTmGS(gsKernAdj float64, b []byte, gs *graphicsState) {
	if ts.tfSize == 0 {
		return
	}
	// gsKernAdj is the net glyph-space advance after kerning adjustments.
	// Convert to text space, then add per-character Tc/Tw on top.
	tx := gsKernAdj/1000.0*ts.tfSize + ts.tcTwAdvance(b)
	adv := matrix3{a: 1, d: 1, e: tx}
	ts.tm = adv.multiply(ts.tm)
	combined := ts.tm.multiply(gs.ctm)
	ts.cursorDevX, ts.cursorDevY = combined.transformPoint(0, 0)
}

// rawBytesAdvance returns the total text-space advance for raw glyph bytes b,
// including glyph widths scaled by tfSize plus Tc per character and Tw per
// 0x20 word-space byte.
func (ts *textState) rawBytesAdvance(b []byte) float64 {
	var tx float64
	tcf := ts.charSpacing * ts.tfSize
	twf := ts.wordSpacing * ts.tfSize
	if ts.currentFont != nil {
		for i := 0; i < len(b); {
			w, n := ts.currentFont.glyphAdvance(b, i)
			tx += w/1000.0*ts.tfSize + tcf
			if n == 1 && b[i] == 0x20 {
				tx += twf
			}
			i += n
		}
	} else {
		// No font loaded: assume 500-unit advance per byte, apply Tc/Tw.
		for _, c := range b {
			tx += 500.0/1000.0*ts.tfSize + tcf
			if c == 0x20 {
				tx += twf
			}
		}
	}
	return tx
}

// tcTwAdvance returns the Tc+Tw contribution (in text space) for raw bytes b,
// without the glyph-width component. Used by advanceTmGS where the glyph
// widths are already accounted for via the kerning-adjusted gsKernAdj.
func (ts *textState) tcTwAdvance(b []byte) float64 {
	if ts.charSpacing == 0 && ts.wordSpacing == 0 {
		return 0
	}
	var tx float64
	tcf := ts.charSpacing * ts.tfSize
	twf := ts.wordSpacing * ts.tfSize
	if ts.currentFont != nil && ts.currentFont.widths != nil {
		// Composite font: step by 1 or 2 bytes depending on whether a 2-byte
		// code exists. Word space only fires on single-byte 0x20.
		for i := 0; i < len(b); {
			n := 1
			if i+1 < len(b) {
				if _, ok := ts.currentFont.widths[(uint16(b[i])<<8)|uint16(b[i+1])]; ok {
					n = 2
				}
			}
			tx += tcf
			if n == 1 && b[i] == 0x20 {
				tx += twf
			}
			i += n
		}
	} else {
		// Simple font or no font: every byte is one character.
		for _, c := range b {
			tx += tcf
			if c == 0x20 {
				tx += twf
			}
		}
	}
	return tx
}

// textSpan holds one contiguous horizontal run of text at a fixed baseline.
// Spans are collected during parsing and sorted into reading order before
// being joined into the final output.
type textSpan struct {
	devY, devX float64
	text       bytes.Buffer
}

// emitGap compares the device-space origin of the next text chunk against the
// current cursor position and decides what separator to emit:
//
//   - |newDevY − cursorDevY| > lineThreshold  →  closes cur, appends to spans,
//     starts a new span at (newDevX, newDevY)
//   - same baseline AND gap > spaceThreshold  →  space written into the current span
//
// It always updates the cursor to (newDevX, newDevY).
func (ts *textState) emitGap(spans *[]textSpan, cur **textSpan, newDevX, newDevY float64) {
	if !ts.tlSet {
		ts.cursorDevX, ts.cursorDevY = newDevX, newDevY
		return
	}

	lineThreshold := ts.fontSize * 0.5
	if lineThreshold < 1 {
		lineThreshold = 1
	}

	dy := ts.cursorDevY - newDevY // positive = moved down the page (PDF y up)
	if dy > lineThreshold || dy < -lineThreshold {
		// Different baseline: seal the current span and open a new one.
		if (*cur).text.Len() > 0 {
			*spans = append(*spans, **cur)
		}
		*cur = &textSpan{devY: newDevY, devX: newDevX}
	} else {
		// Same baseline — emit a space for any visible gap between the end of
		// the last glyph and the start of the next chunk. The threshold of
		// ~20% of font size clears normal kerning (±50–200 glyph units) while
		// catching genuine word gaps. There is no upper cap: large gaps such as
		// those between left- and right-aligned elements on the same header line
		// still produce a single space, which is the correct extraction result.
		spaceThreshold := ts.fontSize * 0.2
		if spaceThreshold < 1 {
			spaceThreshold = 1
		}
		if newDevX-ts.cursorDevX > spaceThreshold {
			(*cur).text.WriteByte(' ')
		}
	}

	ts.cursorDevX, ts.cursorDevY = newDevX, newDevY
}

// parseFloatBytes parses a float from a byte slice without allocating a string.
// It uses an unsafe string view that is valid only for the duration of the call
// — safe here because strconv.ParseFloat does not retain the string.
func parseFloatBytes(b []byte) (float64, error) {
	if len(b) == 0 {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseFloat(unsafe.String(unsafe.SliceData(b), len(b)), 64)
}

// decodeRaw decodes raw PDF string bytes to UTF-8 via the font's tables,
// falling back to Latin-1 when no font is active, writing directly into dst.
func decodeRaw(raw []byte, f *pdfFont, dst *bytes.Buffer) {
	if f == nil {
		decodeLatin1(raw, dst)
		return
	}
	f.decodeBytes(raw, dst)
}

// decodeTJInto decodes the array operand of a TJ operator, writes the decoded
// text to w, and returns:
//
//   - gsKernAdj: net glyph-space advance after folding in all kerning numbers
//     (positive numbers in the array reduce the advance; negative numbers
//     increase it — PDF spec §9.4.3).
//   - allRaw: concatenation of all raw character-code bytes across every string
//     element in the array, used by the caller to apply Tc and Tw.
//
// Word spaces encoded purely as large negative kerning numbers (no 0x20 byte in
// either adjacent string chunk) are emitted as ASCII spaces. The threshold is
// 150 glyph-space units — well above any optical kern pair (~150 max) and well
// below the ~200–500 unit gaps that represent word spaces in practice.
func decodeTJInto(tok []byte, f *pdfFont, w *bytes.Buffer) (gsKernAdj float64, allRaw []byte) {
	tok = bytes.TrimSpace(tok)
	if len(tok) < 2 || tok[0] != '[' || tok[len(tok)-1] != ']' {
		return 0, nil
	}
	inner := tok[1 : len(tok)-1]
	allRaw = make([]byte, 0, len(inner)) // upper-bound pre-allocation

	// lastRaw holds the raw bytes of the most recently decoded string chunk so
	// that the kerning branch can inspect its last byte.
	var lastRaw []byte
	// pendingKernSpace is set when a large negative kern was seen between two
	// string chunks with no adjacent space byte; the space is emitted lazily
	// (before the next chunk) so it is never appended at the very end.
	pendingKernSpace := false

	i := 0
	for i < len(inner) {
		for i < len(inner) && isWhitespaceByte(inner[i]) {
			i++
		}
		if i >= len(inner) {
			break
		}

		if inner[i] == '(' || inner[i] == '<' {
			// Parse the string chunk.
			var raw []byte
			var ok bool
			if inner[i] == '(' {
				end := findClosingParen(inner, i)
				if end < 0 {
					break
				}
				raw, ok = parsePDFString(inner[i : end+1])
				i = end + 1
			} else {
				end := bytes.IndexByte(inner[i+1:], '>') // faster than bytes.Index with slice literal
				if end < 0 {
					break
				}
				raw, ok = parsePDFString(inner[i : i+1+end+1])
				i += 1 + end + 1
			}
			if !ok || len(raw) == 0 {
				continue
			}

			// Emit a pending kern-space unless this chunk starts with 0x20
			// (the space is already present in the text).
			if pendingKernSpace && raw[0] != 0x20 {
				w.WriteByte(' ')
			}
			pendingKernSpace = false

			decodeRaw(raw, f, w)
			if f != nil {
				gsKernAdj += f.rawStringWidth(raw)
			} else {
				gsKernAdj += float64(len(raw)) * 500
			}
			allRaw = append(allRaw, raw...)
			lastRaw = raw

		} else {
			// Kerning number — positive tightens, negative opens.
			start := i
			for i < len(inner) && !isWhitespaceByte(inner[i]) && inner[i] != '(' && inner[i] != '<' {
				i++
			}
			n, err := parseFloatBytes(inner[start:i])
			if err != nil {
				continue
			}
			gsKernAdj -= n

			// A large negative kern (rightward shift > 150 glyph units) between
			// two string chunks with no adjacent 0x20 byte encodes a word space.
			if n < -150 && len(lastRaw) > 0 && lastRaw[len(lastRaw)-1] != 0x20 {
				pendingKernSpace = true
			}
		}
	}
	return gsKernAdj, allRaw
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
		// Strip all whitespace from the hex body in a single pass into a
		// reused stack buffer, avoiding the three-chain ReplaceAll that each
		// allocates. The result is at most (len(s)-2) bytes so it fits in a
		// slice grown once from an initial nil.
		body := s[1 : len(s)-1]
		filtered := make([]byte, 0, len(body))
		for _, b := range body {
			if b != ' ' && b != '\t' && b != '\n' && b != '\r' && b != '\f' {
				filtered = append(filtered, b)
			}
		}
		if len(filtered)%2 != 0 {
			filtered = append(filtered, '0')
		}
		out := make([]byte, len(filtered)/2)
		if _, err := hex.Decode(out, filtered); err != nil {
			return nil, false
		}
		return out, true
	}
	return nil, false
}

// unescapePDFLiteralString unescapes a PDF literal string body (without outer
// parens). If the string contains no backslash escapes the original slice is
// returned directly with no allocation.
func unescapePDFLiteralString(s []byte) []byte {
	// Fast path: no escape sequences — return the slice as-is.
	if bytes.IndexByte(s, '\\') < 0 {
		return s
	}
	buf := make([]byte, 0, len(s)) // upper-bound pre-allocation
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case '\n', '\r':
				// ignore line breaks after the line contiuation marker
				// line splitting inside literal strings is purely optical
			case 'n':
				buf = append(buf, '\n')
			case 'r':
				buf = append(buf, '\r')
			case 't':
				buf = append(buf, '\t')
			case 'b':
				buf = append(buf, '\b')
			case 'f':
				buf = append(buf, '\f')
			case '(', ')', '\\':
				buf = append(buf, s[i])
			default:
				if s[i] >= '0' && s[i] <= '7' {
					val := int(s[i] - '0')
					if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
						i++
						val = val*8 + int(s[i]-'0')
						if i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7' {
							i++
							val = val*8 + int(s[i]-'0')
						}
					}
					buf = append(buf, byte(val))
				} else {
					buf = append(buf, s[i])
				}
			}
		} else {
			buf = append(buf, s[i])
		}
		i++
	}
	return buf
}

// decodeLatin1 converts bytes to UTF-8 using Latin-1, filtering controls,
// writing directly into dst.
func decodeLatin1(b []byte, dst *bytes.Buffer) {
	for _, c := range b {
		r := rune(c)
		if r >= 0x20 && r != 0x7f {
			dst.WriteRune(r)
		}
	}
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
	for line := range strings.SplitSeq(block, "\n") {
		toks := scanHexTokens(line)

		// Array form: <lo> <hi> [<d0> <d1> …]
		if strings.Contains(line, "[") {
			if len(toks) < 2 {
				continue
			}
			lo, err1 := parseHexToken(toks[0])
			hi, err2 := parseHexToken(toks[1])
			if err1 != nil || err2 != nil {
				continue
			}
			// Collect everything between '[' and ']'.
			start := strings.IndexByte(line, '[')
			end := strings.LastIndexByte(line, ']')
			if start < 0 || end <= start {
				continue
			}
			arrToks := scanHexTokens(line[start+1 : end])
			code := lo
			for _, tok := range arrToks {
				dst, err := parseUnicodeHexToken(tok)
				if err == nil {
					m[code] = dst
				}
				if code == hi {
					break
				}
				code++
			}
			continue
		}

		// Scalar form: <lo> <hi> <base>
		if len(toks) < 3 {
			continue
		}
		lo, err1 := parseHexToken(toks[0])
		hi, err2 := parseHexToken(toks[1])
		base, err3 := parseUnicodeHexToken(toks[2])
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		baseRunes := []rune(base)
		if len(baseRunes) == 0 {
			continue
		}
		baseRune := baseRunes[0]
		for code := lo; ; code++ {
			m[code] = string(baseRune + rune(code-lo))
			if code == hi {
				break
			}
		}
	}
}

func parseHexToken(s string) (uint16, error) {
	s = strings.Map(func(r rune) rune {
		if r == '<' || r == '>' || r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
	val, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, err
	}
	return uint16(val), nil
}

func parseUnicodeHexToken(s string) (string, error) {
	// compact and clean the string
	s = strings.Map(func(r rune) rune {
		if r == '<' || r == '>' || r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, s)
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

// tokenIter is a zero-allocation push-style iterator over a PDF content
// stream. Instead of materialising the full token slice upfront it scans
// the raw bytes on demand, yielding one token at a time to a caller-supplied
// yield function.
//
// The iterator is used via the range-over-func pattern introduced in Go 1.23:
//
//	for tok := range tokenIter(content) { … }
//
// Comments (%) are consumed silently and never yielded.
func tokenIter(content []byte) func(yield func([]byte) bool) {
	return func(yield func([]byte) bool) {
		i, n := 0, len(content)

		for i < n {
			// Skip whitespace.
			for i < n && isWhitespaceByte(content[i]) {
				i++
			}
			if i >= n {
				return
			}

			switch content[i] {
			case '%':
				// Comment — skip to end of line.
				for i < n && content[i] != '\n' && content[i] != '\r' {
					i++
				}

			case '(':
				// Literal string: balanced parens, backslash escapes.
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
				if !yield(content[start:i]) {
					return
				}

			case '<':
				if i+1 < n && content[i+1] == '<' {
					// Dict — collect until matching >>, skipping nested hex
					// strings (<...>) so that <</Lang<6465>>> is handled
					// correctly and does not stall the hex-string branch.
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
					if !yield(content[start:i]) {
						return
					}
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
					if !yield(content[start:i]) {
						return
					}
				}

			case '[':
				// Array: collect everything up to the matching ']', honouring
				// nested arrays and literal strings inside the array.
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
				if !yield(content[start:i]) {
					return
				}

			case '/':
				// Name object.
				start := i
				i++
				for i < n && !isWhitespaceByte(content[i]) && !isDelimiter(content[i]) {
					i++
				}
				if !yield(content[start:i]) {
					return
				}

			default:
				// Number, operator, or keyword.
				start := i
				for i < n && !isWhitespaceByte(content[i]) && !isDelimiter(content[i]) {
					i++
				}
				if i > start {
					if !yield(content[start:i]) {
						return
					}
				}
			}
		}
	}
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
