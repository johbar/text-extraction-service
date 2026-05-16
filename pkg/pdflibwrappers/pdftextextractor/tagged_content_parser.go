package pdftextextractor

// content-stream-order variant of the PDF text extractor.
//
// When a page content stream contains BDC or BMC marked-content operators,
// extractPageTextTagged emits text in content-stream order instead of the
// visual (spatially sorted) order used by extractPageText:
//
//   - Runs tagged /Artifact are silently discarded.
//   - /ActualText in a BDC property dictionary replaces the encoded glyph
//     sequence for that marked-content span.
//   - The PDF Structure tree is never consulted.
//
// When no BDC/BMC operators are encountered the output is identical to
// extractPageText (visual reading order).

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/johbar/pdfcpu-lite/pkg/pdfcpu/model"
)

// ---------------------------------------------------------------------------
// Marked-content tag stack
// ---------------------------------------------------------------------------

// tagEntry records one level of the PDF marked-content operator nesting stack.
// Entries are pushed by BDC/BMC and popped by EMC.
type tagEntry struct {
	// name is the tag role without the leading "/" (e.g. "P", "Span", "Artifact").
	name string
	// actualText is the decoded UTF-8 replacement string (valid when hasActualText).
	actualText string
	// mcid is the value of the /MCID property; -1 when the property is absent.
	mcid int
	// cursorDevX when this tag was entered
	devX float64
	devY float64
	// hasActualText is true when the BDC property dict contained /ActualText.
	hasActualText bool
}

var spanBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func getSpanBuf() *bytes.Buffer {
	b := spanBufPool.Get().(*bytes.Buffer)
	b.Reset()
	return b
}

func putSpanBuf(b *bytes.Buffer) {
	// Don't retain pathologically large buffers in the pool.
	if b != nil && b.Cap() <= 64<<10 {
		spanBufPool.Put(b)
	}
}

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

// extractPageTextTaggedOrder is the tagged variant of extractPageText.
//
// The caller interface is identical to extractPageText; only the span-ordering
// strategy changes when marked-content operators are detected.
func extractPageTextTaggedOrder(ctx *model.Context, pageNr int) (*bytes.Buffer, error) {
	if pageNr < 1 || pageNr > ctx.PageCount {
		return nil, fmt.Errorf("extractPageTextTaggedOrder: invalid page number %d (document has %d pages)", pageNr, ctx.PageCount)
	}
	pageDict, _, inhPAttrs, err := ctx.XRefTable.PageDict(pageNr, true /*consolidateRes*/)
	if err != nil {
		return nil, fmt.Errorf("extractPageTextTaggedOrder: page %d: %w", pageNr, err)
	}
	if pageDict == nil {
		return nil, fmt.Errorf("extractPageTextTaggedOrder: page %d not found", pageNr)
	}
	content, err := ctx.XRefTable.PageContent(pageDict, pageNr)
	if err != nil {
		if err == model.ErrNoContent {
			return nil, nil
		}
		return nil, fmt.Errorf("extractPageTextTaggedOrder: page %d content: %w", pageNr, err)
	}
	fontMap := buildFontMap(ctx.XRefTable, inhPAttrs.Resources)
	xobjMap := buildXObjMap(ctx.XRefTable, inhPAttrs.Resources)

	text, err := extractTextFromContentTagged(content, fontMap, xobjMap)
	if err != nil {
		return nil, fmt.Errorf("extractPageTextTaggedOrder: page %d parse: %w", pageNr, err)
	}
	return &text, nil
}

// ---------------------------------------------------------------------------
// Core extraction logic
// ---------------------------------------------------------------------------

// extractTextFromContentTagged drives parseContentStreamTagged and joins the
// resulting spans either in content-stream order (tagged PDFs) or visual
// reading order (untagged fallback).
func extractTextFromContentTagged(content []byte, fontMap map[string]*pdfFont, xobjMap map[string]xObject) (bytes.Buffer, error) {
	spans := make([]textSpan, 0, 64)
	cur := &textSpan{text: getSpanBuf()}
	tagged := false

	cursorDevX := parseContentStreamTagged(content, fontMap, xobjMap, newGraphicsState(), &spans, &cur, &tagged)

	// Seal whatever the last operator left in the open span.
	if cur.text.Len() > 0 {
		(*cur).devXEnd = cursorDevX
		spans = append(spans, *cur)
	}

	if !tagged {
		// No marked-content operators found: sort into visual reading order,
		// giving output identical to the untagged extractor.
		slices.SortFunc(spans, func(a, b textSpan) int {
			if a.devY != b.devY {
				if a.devY > b.devY {
					return -1
				}
				return 1
			}
			if a.devX < b.devX {
				return -1
			}
			if a.devX > b.devX {
				return 1
			}
			return 0
		})
	}
	// When tagged, spans are already in content-stream order; no sort needed.

	// Join spans, inserting whitespace inferred from device-space coordinates.
	// This logic works correctly in both sorted and stream order because each
	// span carries its own devX/devY regardless of how they were ordered.
	var out bytes.Buffer
	for k, sp := range spans {
		if k == 0 {
			out.Write(sp.text.Bytes())
			putSpanBuf(sp.text)
			continue
		}
		prev := spans[k-1]
		dy := prev.devY - sp.devY
		if dy > 1 || dy < -1 {
			out.WriteByte('\n')
			// fixed space threshold here because textstate
			// and font are not available.
		} else if sp.devX-prev.devXEnd > 1 {
			out.WriteByte(' ')
		}
		out.Write(sp.text.Bytes())
		putSpanBuf(sp.text)
	}
	return out, nil
}

// parseContentStreamTagged is the tagged variant of parseContentStream.
//
// It handles all operators of the base parser and additionally:
//
//	BMC  — /TagName BMC
//	       Push a tag entry; set *tagged = true.  Increment artifactDepth for
//	       /Artifact tags.
//
//	BDC  — /TagName propdict_or_name BDC
//	       Push a tag entry; extract /MCID and /ActualText from inline property
//	       dicts.  Named property references (plain /Name operands that point to
//	       a Properties resource entry) are noted but not resolved.
//
//	EMC  — Pop the tag stack.  When the popped entry had /ActualText, write the
//	       decoded string to the current span now that all enclosed glyphs have
//	       been suppressed.
//
// Text showing operators (Tj TJ ' ") are silently suppressed when inside an
// Artifact run (artifactDepth > 0) or inside an ActualText range
// (actualTextDepth > 0).  The text matrix is always advanced so cursor
// tracking remains accurate after the suppressed run.
//
// Form XObjects invoked inside an Artifact run are skipped entirely.
// It returns the last cursor X position.
func parseContentStreamTagged(
	content []byte,
	fontMap map[string]*pdfFont,
	xobjMap map[string]xObject,
	gs graphicsState,
	spans *[]textSpan,
	cur **textSpan,
	tagged *bool,
) float64 {
	ts := &textState{fontMap: fontMap}

	const winSize = 8
	const winMask = winSize - 1
	var win [winSize][]byte
	pos := 0
	atBack := func(n int) []byte { return win[(pos-n)&winMask] }

	// Marked-content state ------------------------------------------------
	var tagStack []tagEntry
	artifactDepth := 0   // > 0 while inside one or more /Artifact tags
	actualTextDepth := 0 // > 0 while inside a tag that carries /ActualText

	// throwaway is a reused discard buffer for suppressed glyph output, avoiding
	// allocating a fresh buffer on every suppressed showing operator.
	throwaway := getSpanBuf()
	defer putSpanBuf(throwaway)
	// sink returns the span write buffer when output is live, or the throwaway
	// buffer when inside an Artifact or ActualText suppression range.
	//
	// Routing through sink means every showing operator can be written as a
	// single call without an explicit suppression check in each case arm:
	//   decodeRaw(raw, ts.currentFont, sink())
	// When suppressed, decodeTJInto / decodeRaw still return correct width and
	// allRaw values, which are forwarded to advanceTm / advanceTmGS so that the
	// text matrix advances correctly despite the discarded output.
	sink := func() *bytes.Buffer {
		if artifactDepth > 0 || actualTextDepth > 0 {
			throwaway.Reset()
			return throwaway
		}
		return (*cur).text
	}

	// emitGapOrTrack calls ts.emitGap (which may seal spans and write inter-word
	// spaces) when outside an Artifact run.  Inside an Artifact it merely updates
	// the cursor so that gap detection after the run ends remains accurate.
	//
	// We intentionally do NOT call emitGap inside Artifact runs:
	//   • emitGap can write a space byte into (*cur).text — which would corrupt
	//     the span that will receive the next real (non-artifact) text.
	//   • emitGap can seal and append the current span — premature for content
	//     that is not yet finished.
	// Tracking the cursor manually preserves enough information for the first
	// real emitGap call after the Artifact ends to fire correctly.
	emitGapOrTrack := func(newDevX, newDevY float64) {
		if artifactDepth > 0 || actualTextDepth > 0 {
			ts.cursorDevX = newDevX
			ts.cursorDevY = newDevY
			return
		}
		ts.emitGap(spans, cur, newDevX, newDevY)
	}

	for tok := range tokenIter(content) {
		s := unsafe.String(&tok[0], len(tok))
		switch s {

		// -------------------------------------------------------------------
		// Graphics state operators
		// -------------------------------------------------------------------

		case "q":
			gs.push()

		case "Q":
			gs.pop()
			ts.updateFontSize(&gs)

		case "cm":
			if pos >= 7 {
				a, ea := parseFloatBytes(atBack(6))
				b, eb := parseFloatBytes(atBack(5))
				c, ec := parseFloatBytes(atBack(4))
				d, ed := parseFloatBytes(atBack(3))
				e, ee := parseFloatBytes(atBack(2))
				f, ef := parseFloatBytes(atBack(1))
				if ea == nil && eb == nil && ec == nil && ed == nil && ee == nil && ef == nil {
					gs.ctm = matrix3{a: a, b: b, c: c, d: d, e: e, f: f}.multiply(gs.ctm)
					ts.updateFontSize(&gs)
				}
			}

		// -------------------------------------------------------------------
		// Marked-content operators
		// -------------------------------------------------------------------

		case "BMC":
			// Syntax: /TagName BMC  (one operand)
			*tagged = true
			if pos >= 2 {
				name := string(stripSlash(atBack(1)))
				tagStack = append(tagStack, tagEntry{
					name: name,
					mcid: -1,
					devX: ts.cursorDevX,
					devY: ts.cursorDevY,
				})
				if name == "Artifact" {
					artifactDepth++
				}
			}

		case "BDC":
			// Syntax: /TagName propdict_or_name BDC  (two operands)
			// propdict is either an inline dict "<<...>>" or a /Name that refers
			// to an entry in the page's Properties resource dictionary.
			// Only inline dicts are parsed here; named references are skipped.
			*tagged = true
			if pos >= 3 {
				name := string(stripSlash(atBack(2)))
				mcid, actualText, hasActualText := parseMarkedContentProps(atBack(1))
				tagStack = append(tagStack, tagEntry{
					name:          name,
					mcid:          mcid,
					hasActualText: hasActualText,
					actualText:    actualText,
					devX:          ts.cursorDevX,
					devY:          ts.cursorDevY,
				})
				if name == "Artifact" {
					artifactDepth++
				}
				if hasActualText {
					actualTextDepth++
				}
			}

		case "EMC":
			if len(tagStack) > 0 {
				top := tagStack[len(tagStack)-1]
				tagStack = tagStack[:len(tagStack)-1]
				if top.name == "Artifact" && artifactDepth > 0 {
					artifactDepth--
					if artifactDepth == 0 {
						dy := ts.cursorDevY - top.devY
						lineThreshold := ts.fontSize * 0.5
						if lineThreshold < 1 {
							lineThreshold = 1
						}
						if dy > -lineThreshold && dy < lineThreshold &&
							ts.cursorDevX > top.devX+ts.fontSize*0.2 {
							(*cur).text.WriteByte(' ')
						}
					}
				}
				if top.hasActualText && actualTextDepth > 0 {
					actualTextDepth--
					if artifactDepth == 0 {
						(*cur).text.WriteString(top.actualText)
					}
				}
			}

		// -------------------------------------------------------------------
		// XObject invocation
		// -------------------------------------------------------------------

		case "Do":
			// Skip Form XObjects that are part of an Artifact run: their content
			// is semantically part of the artifact and should not appear in the
			// extracted text.
			if artifactDepth > 0 {
				break
			}
			if pos >= 2 {
				if xobj, ok := xobjMap[string(stripSlash(atBack(1)))]; ok {
					// Seal the current span before recursing so the XObject's
					// spans sort on their own device coordinates.
					ts.sealCur(spans, cur, ts.cursorDevX, ts.cursorDevY)
					// Build the child CTM: xobj.matrix × parent CTM.
					childGS := graphicsState{ctm: xobj.matrix.multiply(gs.ctm)}
					// Merge font maps; XObject fonts shadow parent fonts when names collide.
					childFonts := fontMap
					if len(xobj.fontMap) > 0 {
						merged := make(map[string]*pdfFont, len(fontMap)+len(xobj.fontMap))
						maps.Copy(merged, fontMap)
						maps.Copy(merged, xobj.fontMap)
						childFonts = merged
					}
					devX := parseContentStreamTagged(xobj.content, childFonts, xobj.xobjMap, childGS, spans, cur, tagged)
					// Seal whatever the XObject left so it doesn't bleed into the
					// parent stream's next span.
					if (*cur).text.Len() > 0 {
						(*cur).devXEnd = devX
						*spans = append(*spans, **cur)
						*cur = &textSpan{text: getSpanBuf()}
					}
				}
			}

		// -------------------------------------------------------------------
		// Text object delimiters
		// -------------------------------------------------------------------

		case "BT":
			ts.inBT = true
			ts.tlm = identityMatrix()
			ts.tm = identityMatrix()
			ts.updateFontSize(&gs)

		case "ET":
			ts.inBT = false

		// -------------------------------------------------------------------
		// Text state operators
		// -------------------------------------------------------------------

		case "Tf":
			if pos >= 3 {
				if f, ok := fontMap[string(stripSlash(atBack(2)))]; ok {
					ts.currentFont = f
				} else {
					ts.currentFont = nil
				}
				ts.tfSize, _ = parseFloatBytes(atBack(1))
				if ts.tfSize < 0 {
					ts.tfSize = -ts.tfSize
				}
				ts.updateFontSize(&gs)
			}

		case "TL":
			if pos >= 2 {
				ts.leading, _ = parseFloatBytes(atBack(1))
			}

		case "Tc":
			if pos >= 2 {
				ts.charSpacing, _ = parseFloatBytes(atBack(1))
			}

		case "Tw":
			if pos >= 2 {
				ts.wordSpacing, _ = parseFloatBytes(atBack(1))
			}

		// -------------------------------------------------------------------
		// Text positioning operators
		// -------------------------------------------------------------------

		case "Tm":
			if ts.inBT && pos >= 7 {
				a, ea := parseFloatBytes(atBack(6))
				b, eb := parseFloatBytes(atBack(5))
				c, ec := parseFloatBytes(atBack(4))
				d, ed := parseFloatBytes(atBack(3))
				e, ee := parseFloatBytes(atBack(2))
				f, ef := parseFloatBytes(atBack(1))
				if ea == nil && eb == nil && ec == nil && ed == nil && ee == nil && ef == nil {
					mat := matrix3{a: a, b: b, c: c, d: d, e: e, f: f}
					newDevX, newDevY := mat.multiply(gs.ctm).transformPoint(0, 0)
					ts.setTm(mat, &gs)
					emitGapOrTrack(newDevX, newDevY)
					if newDevX > ts.cursorDevX {
						ts.cursorDevX = newDevX
					}
					ts.cursorDevY = newDevY
				}
			}

		case "Td", "TD":
			if ts.inBT && pos >= 3 {
				ty, ey := parseFloatBytes(atBack(1))
				tx, ex := parseFloatBytes(atBack(2))
				if ey == nil && ex == nil {
					if bytes.Equal(tok, []byte{'T', 'D'}) {
						ts.leading = -ty
					}
					ts.applyTd(tx, ty, &gs)
					newDevX, newDevY := ts.deviceOrigin(&gs)
					emitGapOrTrack(newDevX, newDevY)
					if newDevX > ts.cursorDevX {
						ts.cursorDevX = newDevX
					}
					ts.cursorDevY = newDevY
				}
			}

		case "T*":
			if ts.inBT {
				ts.applyTd(0, -ts.leading, &gs)
				newDevX, newDevY := ts.deviceOrigin(&gs)
				emitGapOrTrack(newDevX, newDevY)
				if newDevX > ts.cursorDevX {
					ts.cursorDevX = newDevX
				}
				ts.cursorDevY = newDevY
			}

		// -------------------------------------------------------------------
		// Text showing operators
		// -------------------------------------------------------------------
		//
		// All four operators route their decoded output through sink(), which
		// transparently discards to a throwaway buffer when inside an Artifact
		// or ActualText suppression range.  Width computation and Tm advancement
		// always execute so cursor tracking stays accurate.

		case "Tj":
			if ts.inBT && pos >= 2 {
				if raw, ok := parsePDFString(atBack(1)); ok {
					decodeRaw(raw, ts.currentFont, sink())
					ts.advanceTm(raw, &gs)
				}
			}

		case "'":
			// Move to the next line (Td 0 -leading) then show string.
			if ts.inBT && pos >= 2 {
				ts.applyTd(0, -ts.leading, &gs)
				newDevX, newDevY := ts.deviceOrigin(&gs)
				emitGapOrTrack(newDevX, newDevY)
				if newDevX > ts.cursorDevX {
					ts.cursorDevX = newDevX
				}
				ts.cursorDevY = newDevY
				if raw, ok := parsePDFString(atBack(1)); ok {
					decodeRaw(raw, ts.currentFont, sink())
					ts.advanceTm(raw, &gs)
				}
			}

		case `"`:
			// Set word/char spacing, move to next line, then show string.
			if ts.inBT && pos >= 4 {
				ts.wordSpacing, _ = parseFloatBytes(atBack(3))
				ts.charSpacing, _ = parseFloatBytes(atBack(2))
				ts.applyTd(0, -ts.leading, &gs)
				newDevX, newDevY := ts.deviceOrigin(&gs)
				emitGapOrTrack(newDevX, newDevY)
				if newDevX > ts.cursorDevX {
					ts.cursorDevX = newDevX
				}
				ts.cursorDevY = newDevY
				if raw, ok := parsePDFString(atBack(1)); ok {
					decodeRaw(raw, ts.currentFont, sink())
					ts.advanceTm(raw, &gs)
				}
			}

		case "TJ":
			if ts.inBT && pos >= 2 {
				gsAdv, tcTwAdv := parseTJArray(atBack(1), ts, sink())
				ts.advanceTmGS(gsAdv, tcTwAdv, &gs)
			}
		}

		win[pos&winMask] = tok
		pos++
	}
	return ts.cursorDevX
}

// ---------------------------------------------------------------------------
// Marked-content property helpers
// ---------------------------------------------------------------------------

// parseMarkedContentProps extracts /MCID and /ActualText from the property
// operand of a BDC operator.
//
// The operand tok is either an inline dictionary token of the form "<<...>>" or
// a plain name token ("/PropName") that refers to a named entry in the page's
// Properties resource dictionary.  Only inline dictionaries are handled here;
// named references require a resource lookup that is not performed by this
// function, so they return mcid == -1 and hasActualText == false.
//
// The dictionary body is tokenised with the same tokenIter used by the content
// stream parser, so nested strings and nested dicts (e.g. attribute objects) are
// consumed correctly.
func parseMarkedContentProps(tok []byte) (mcid int, actualText string, hasActualText bool) {
	mcid = -1
	tok = bytes.TrimSpace(tok)
	// Named property references start with '/' and are not inline dicts.
	if len(tok) < 4 || tok[0] != '<' || tok[1] != '<' {
		return
	}
	// Strip the outer << and >> to get the dict body.
	inner := tok[2 : len(tok)-2]
	var key string
	for t := range tokenIter(inner) {
		s := unsafe.String(&t[0], len(t))
		if len(s) > 0 && s[0] == '/' {
			key = s[1:] // PDF name: advance to its value on the next iteration
			continue
		}
		switch key {
		case "MCID":
			if v, err := parseFloatBytes(t); err == nil {
				mcid = int(v)
			}
		case "ActualText":
			if raw, ok := parsePDFString(t); ok {
				actualText = decodeActualText(raw)
				hasActualText = true
			}
		}
		key = "" // reset: the current token was a value, not a key
	}
	return
}

// decodeActualText decodes the raw bytes of an /ActualText PDF string (already
// unescaped/un-hexed by parsePDFString) into a UTF-8 Go string.
//
// Strings that begin with the UTF-16BE byte-order mark 0xFE 0xFF are decoded
// via utf16.Decode. All other byte sequences are interpreted as PDFDocEncoding
// (a superset of Latin-1), with C0 and DEL control bytes filtered out.
func decodeActualText(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		// UTF-16BE with BOM.
		u16 := make([]uint16, 0, (len(b)-2)/2)
		for i := 2; i+1 < len(b); i += 2 {
			decoded := (uint16(b[i]) << 8) | uint16(b[i+1])
			// translate non-breaking spaces and tabs to normal white space
			if decoded == 0xA0 || decoded == '\t' {
				decoded = ' '
			}
			u16 = append(u16, decoded)
		}
		return string(utf16.Decode(u16))
	}
	// PDFDocEncoding / Latin-1 fallback.
	var sb strings.Builder
	sb.Grow(len(b))
	for _, c := range b {
		if c >= 0x20 && c != 0x7f {
			// translate non-breaking spaces and tabs to normal white space
			if c == 0xA0 || c == '\t' {
				sb.WriteByte(' ')
				continue
			}
			sb.WriteByte(c)
		}
	}
	return sb.String()
}
