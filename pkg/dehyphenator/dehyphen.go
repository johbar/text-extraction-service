/*
Package dehyphenator implements an algorithm for de-hyphenating German text.

German includes a lot of compounds, some involving hyphens, lowercase and
uppercase characters ("US-amerikanisch" "EU-Institution"). This package aims to
preserve hyphens when they are partof a compound and to remove them at the end
of lines whenever they are not.
Not sure if it is of any use when working with other languages.
*/
package dehyphenator

import (
	"bytes"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// tailSize is the number of bytes kept in the tail buffer at all times once a
// line is underway. 16 bytes comfortably cover 2 max-width UTF-8 runes (4 bytes
// each) plus a few trailing spaces, which is all the look-back the algorithm
// ever needs.
const tailSize = 16

// DehyphenWriter implements [io.WriteCloser]. Write arbitrary chunks of text
// to it; it will dehyphenate on the fly and forward the result to the
// underlying writer. Close must be called when writing is done so that any
// text that was not yet terminated by a newline is flushed.
//
// Bytes that cannot influence dehyphenation decisions are streamed straight
// through to the underlying writer; only a small tail (≤ tailSize bytes) is
// buffered at any time.
type DehyphenWriter struct {
	removeNewlines bool

	out io.Writer

	// tail holds the last few bytes of the current incomplete line. It is at
	// most tailSize bytes while lineStarted is true; before that it may grow
	// slightly while we search for the first content rune.
	tail []byte

	// lastHyphen is the hyphen rune stripped from the previous line's end, or
	// 0x00 when no such hyphen is pending.
	lastHyphen rune

	// lineStarted is true once we have written at least one byte of the current
	// line to out (leading whitespace trimmed, lastHyphen decision resolved).
	lineStarted bool

	// prevContentRune is the last non-space rune that was written to out for
	// the current line. It is needed when the tail shrinks to a single hyphen
	// rune, so we can still determine whether the rune immediately before the
	// hyphen was uppercase — even though that rune has already been flushed.
	prevContentRune rune
}

// New returns a new DehyphenWriter that writes to out. When removeNewlines is
// true, newlines in the output are replaced with spaces.
func New(out io.Writer, removeNewlines bool) *DehyphenWriter {
	return &DehyphenWriter{out: out, removeNewlines: removeNewlines}
}

// Write implements [io.Writer]. Bytes that cannot affect dehyphenation are
// forwarded to the underlying writer immediately; only a small constant-sized
// tail is retained between calls.
func (d *DehyphenWriter) Write(p []byte) (int, error) {
	d.tail = append(d.tail, p...)

	for {
		idx := bytes.IndexByte(d.tail, '\n')
		if idx >= 0 {
			if err := d.finishLine(idx); err != nil {
				return 0, err
			}
			d.tail = d.tail[idx+1:]
			d.lineStarted = false
			d.prevContentRune = 0x00
			continue
		}

		// No newline in the buffer. Advance the head if not done yet, then
		// flush every byte we are certain we won't need to revisit.
		if !d.lineStarted {
			ok, err := d.advanceHead()
			if err != nil {
				return 0, err
			}
			if !ok {
				break // waiting for enough bytes to decode the first rune
			}
		}
		if err := d.flushSafe(); err != nil {
			return 0, err
		}
		break
	}

	return len(p), nil
}

// Close implements [io.Closer]. It flushes any remaining buffered bytes as the
// final (newline-less) line. The underlying writer is not closed.
func (d *DehyphenWriter) Close() error {
	if len(d.tail) > 0 {
		if err := d.finishLine(len(d.tail)); err != nil {
			return err
		}
		d.tail = nil
	}
	return nil
}

// advanceHead trims leading whitespace from d.tail, resolves the lastHyphen
// decision, and writes the first content rune to out. It returns (true, nil)
// on success or (false, nil) when more bytes are needed to decode a rune.
func (d *DehyphenWriter) advanceHead() (bool, error) {
	// Skip leading whitespace one rune at a time.
	i := 0
	for i < len(d.tail) {
		r, size := utf8.DecodeRune(d.tail[i:])
		if r == utf8.RuneError && size == 1 {
			d.tail = d.tail[i:] // drop consumed whitespace, wait for more
			return false, nil
		}
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	if i == len(d.tail) {
		d.tail = d.tail[:0]
		return false, nil // only whitespace seen so far
	}

	// Decode the first content rune.
	r, size := utf8.DecodeRune(d.tail[i:])
	if r == utf8.RuneError && size == 1 {
		d.tail = d.tail[i:] // drop whitespace prefix, wait for more
		return false, nil
	}

	// Resolve lastHyphen: restore it only when this line starts with uppercase.
	if d.lastHyphen != 0x00 {
		if unicode.IsUpper(r) {
			if _, err := io.WriteString(d.out, string(d.lastHyphen)); err != nil {
				return false, err
			}
		}
		d.lastHyphen = 0x00
	}

	// Drop leading whitespace but keep the first content rune in tail for
	// flushSafe / finishLine to handle. Writing it here would prevent
	// finishLine from detecting a hyphen-only line (single '-' with nothing
	// before or after it), because lineStarted would already be true and the
	// rune would be gone from the buffer.
	d.tail = d.tail[i:]
	d.lineStarted = true
	return true, nil
}

// flushSafe writes all but the last tailSize bytes of d.tail straight to out,
// keeping only the minimum suffix needed for end-of-line inspection.
func (d *DehyphenWriter) flushSafe() error {
	safe := len(d.tail) - tailSize
	if safe <= 0 {
		return nil
	}
	chunk := d.tail[:safe]
	if _, err := d.out.Write(chunk); err != nil {
		return err
	}
	// Track the last non-space rune we wrote, so finishLine can still inspect
	// the character before a trailing hyphen even if it was in the safe region.
	for s := string(chunk); len(s) > 0; {
		r, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
		if !unicode.IsSpace(r) {
			d.prevContentRune = r
			break
		}
	}
	d.tail = d.tail[safe:]
	return nil
}

// finishLine processes d.tail[:nlIdx] as the end of the current line (the '\n'
// at d.tail[nlIdx] is not included).
func (d *DehyphenWriter) finishLine(nlIdx int) error {
	content := d.tail[:nlIdx]

	if !d.lineStarted {
		// The whole line is still in the buffer — handle it from scratch.
		return d.processWholeLine(string(content))
	}

	// Safe bytes for this line were already written. Inspect the remaining tail.
	trimmed := strings.TrimRightFunc(string(content), unicode.IsSpace)
	runes := []rune(trimmed)

	// Hyphen-only line in the streaming path: advanceHead ran (lineStarted=true)
	// but nothing has been written to out yet (prevContentRune==0x00) and the
	// only rune is a hyphen. Skip silently, matching processWholeLine behaviour.
	if d.prevContentRune == 0x00 && len(runes) == 1 && isHyphen(runes[0]) {
		return nil
	}

	if len(runes) == 0 {
		// Everything significant was already flushed; just close the line.
		return d.writeSeparator()
	}
	return d.applyEndOfLineLogic(runes)
}

// processWholeLine handles a line that arrived entirely in the buffer
// (lineStarted == false): it trims both ends and applies all logic from
// scratch.
func (d *DehyphenWriter) processWholeLine(raw string) error {
	line := strings.TrimSpace(raw)
	runes := []rune(line)
	n := len(runes)

	if n == 0 || (n == 1 && isHyphen(runes[0])) {
		// Skip empty and hyphen-only lines.
		// Do NOT clear lastHyphen — it must survive across blank lines, matching
		// the behaviour of the original scanner-based implementation.
		return nil
	}

	if d.lastHyphen != 0x00 {
		if unicode.IsUpper(runes[0]) {
			if _, err := io.WriteString(d.out, string(d.lastHyphen)); err != nil {
				return err
			}
		}
		d.lastHyphen = 0x00
	}

	return d.applyEndOfLineLogic(runes)
}

// applyEndOfLineLogic writes runes to out and applies hyphen handling.
// runes is either the entire line (when called from processWholeLine) or just
// the unwritten tail (when called from finishLine after a safe flush).
func (d *DehyphenWriter) applyEndOfLineLogic(runes []rune) error {
	n := len(runes)

	if !isHyphen(runes[n-1]) {
		if _, err := io.WriteString(d.out, string(runes)); err != nil {
			return err
		}
		return d.writeSeparator()
	}

	// The line ends with a hyphen. Determine the rune immediately before it.
	var beforeHyphen rune
	if n >= 2 {
		beforeHyphen = runes[n-2]
	} else {
		// The tail is just the hyphen itself; the rune before it was already
		// flushed as a safe byte. Fall back to prevContentRune.
		beforeHyphen = d.prevContentRune
	}

	if unicode.IsUpper(beforeHyphen) {
		// Abbreviation-style compound (e.g. "E-" in "E-Mail"): keep the hyphen
		// and suppress the separator so the next line is appended directly.
		_, err := io.WriteString(d.out, string(runes))
		return err
	}

	// Line-break hyphen: strip it and remember it for the next line.
	d.lastHyphen = runes[n-1]
	_, err := io.WriteString(d.out, string(runes[:n-1]))
	return err
}

func (d *DehyphenWriter) writeSeparator() error {
	sep := []byte{'\n'}
	if d.removeNewlines {
		sep = []byte{' '}
	}
	_, err := d.out.Write(sep)
	return err
}

func isHyphen(char rune) bool {
	return unicode.Is(unicode.Hyphen, char)
}
