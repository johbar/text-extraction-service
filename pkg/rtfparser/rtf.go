package rtfparser

// Package rtfparse provides a streaming RTF (Rich Text Format) to plain text converter.
// It operates on io.Reader/io.Writer interfaces and requires no full-document buffering.

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parser converts RTF input to plain text in a streaming fashion.
type Parser struct {
	r   *bufio.Reader
	w   *bufio.Writer
	err error

	// group stack
	stack []*group

	// unicode skip count (from \ucN)
	unicodeSkip int
}

// group holds the state for a single RTF group { ... }
type group struct {
	// destination name (e.g. "fonttbl", "colortbl", "info", "pict", ...)
	destination string
	// whether this group (or any ancestor) is a skipped destination
	skip bool
	// code page for \'xx escapes
	codePage int
	// unicode replacement character count
	ucValue int
}

// ignoredDestinations lists RTF destinations whose content should not appear in
// plain-text output.
var ignoredDestinations = map[string]bool{
	"fonttbl":            true,
	"colortbl":           true,
	"stylesheet":         true,
	"info":               true,
	"pict":               true,
	"object":             true,
	"objdata":            true,
	"result":             true,
	"fldinst":            true,
	"fldrslt":            false, // we WANT field results
	"shppict":            true,
	"nonshppict":         true,
	"themedata":          true,
	"colorschememapping": true,
	"datastore":          true,
	"latentstyles":       true,
	"revtbl":             true,
	"rsidtbl":            true,
	"listtext":           true,
}

// NewParser creates a new streaming RTF parser that reads from r and writes
// plain text to w.
func NewParser(r io.Reader, w io.Writer) *Parser {
	return &Parser{
		r: bufio.NewReaderSize(r, 32*1024),
		w: bufio.NewWriterSize(w, 32*1024),
	}
}

// Parse runs the full conversion. Returns the first error encountered.
func (p *Parser) Parse() error {
	// Push a root group
	p.push(&group{codePage: 1252, ucValue: 1})

	for {
		b, err := p.r.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("rtfparse: read error: %w", err)
		}

		switch b {
		case '{':
			p.openGroup()
		case '}':
			p.closeGroup()
		case '\\':
			if err := p.parseControl(); err != nil {
				return err
			}
		case '\r', '\n':
			// bare newlines in RTF are ignored (not content)
		default:
			p.writeChar(b)
		}
	}

	return p.w.Flush()
}

// openGroup duplicates the top group state onto the stack.
func (p *Parser) openGroup() {
	top := p.top()
	g := &group{
		destination: top.destination,
		skip:        top.skip,
		codePage:    top.codePage,
		ucValue:     top.ucValue,
	}
	p.stack = append(p.stack, g)
}

// closeGroup pops the stack.
func (p *Parser) closeGroup() {
	if len(p.stack) > 1 {
		p.stack = p.stack[:len(p.stack)-1]
	}
}

// top returns the current group (top of stack).
func (p *Parser) top() *group {
	return p.stack[len(p.stack)-1]
}

// push adds a group to the stack.
func (p *Parser) push(g *group) {
	p.stack = append(p.stack, g)
}

// parseControl reads a control word or symbol after the leading backslash.
func (p *Parser) parseControl() error {
	b, err := p.r.ReadByte()
	if err != nil {
		return fmt.Errorf("rtfparse: unexpected EOF in control: %w", err)
	}

	switch {
	case b == '\'':
		return p.parseHexChar()
	case b == '*':
		// \* marks the next destination as ignorable
		p.top().skip = true
		return nil
	case b == '\\':
		p.writeChar('\\')
		return nil
	case b == '{':
		p.writeChar('{')
		return nil
	case b == '}':
		p.writeChar('}')
		return nil
	case b == '\r' || b == '\n':
		// \<newline> is a paragraph delimiter
		p.writeParagraph()
		return nil
	case b == '-':
		// optional hyphen — skip
		return nil
	case b == '_':
		// non-breaking hyphen
		p.writeRune('\u2011')
		return nil
	case b == '~':
		// non-breaking space
		p.writeRune('\u00a0')
		return nil
	case b == '|':
		// formula character — skip
		return nil
	case b == ':':
		// index sub-entry — skip
		return nil
	case isLetter(b):
		return p.parseWord(b)
	default:
		// unknown symbol, skip
		return nil
	}
}

// parseWord reads a full control word starting with the first letter already consumed.
func (p *Parser) parseWord(first byte) error {
	var buf [64]byte
	buf[0] = first
	n := 1

	// Read remaining letters — RTF control words are always lowercase.
	// An uppercase letter is content, not part of the word; unread it.
	for {
		b, err := p.r.ReadByte()
		if err != nil {
			break
		}
		if isLetter(b) {
			if n < len(buf) {
				buf[n] = b
				n++
			}
		} else {
			// May be a numeric parameter
			if b == '-' || isDigit(b) {
				return p.parseWordWithParam(string(buf[:n]), b)
			}
			// Space is consumed as delimiter; everything else is unread.
			if b != ' ' {
				_ = p.r.UnreadByte()
			}
			break
		}
	}

	p.applyWord(string(buf[:n]), 0, false)
	return nil
}

// parseWordWithParam reads the numeric parameter for a control word.
func (p *Parser) parseWordWithParam(word string, sign byte) error {
	negative := sign == '-'
	var digits [20]byte
	n := 0
	if isDigit(sign) {
		digits[0] = sign
		n = 1
	}

	for {
		b, err := p.r.ReadByte()
		if err != nil {
			break
		}
		if isDigit(b) {
			if n < len(digits) {
				digits[n] = b
				n++
			}
		} else {
			if b != ' ' {
				_ = p.r.UnreadByte()
			}
			break
		}
	}

	param := 0
	if n > 0 {
		v, _ := strconv.Atoi(string(digits[:n]))
		param = v
	}
	if negative {
		param = -param
	}

	p.applyWord(word, param, true)
	return nil
}

// applyWord processes a recognized control word.
func (p *Parser) applyWord(word string, param int, hasParam bool) {
	top := p.top()

	switch word {
	// --- Destinations ---
	case "fonttbl", "colortbl", "stylesheet", "info", "pict",
		"object", "objdata", "shppict", "nonshppict",
		"themedata", "colorschememapping", "datastore",
		"latentstyles", "revtbl", "rsidtbl", "fldinst", "listtext":
		top.destination = word
		if ignoredDestinations[word] {
			top.skip = true
		}

	case "fldrslt":
		top.destination = word
		top.skip = false // field results ARE output

	// --- Paragraph / line breaks ---
	case "par": //, "pard":
		p.writeParagraph()
	case "line":
		p.writeRune('\n')
	case "tab":
		p.writeRune('\t')
		// p.writeRune(' ')
	case "page", "column":
		p.writeParagraph()

	// --- Unicode ---
	case "u":
		// \uN emits the Unicode code point N, then skips ucValue chars
		r := rune(param)
		if r < 0 {
			r += 65536 // RTF signed 16-bit
		}
		p.writeRune(r)
		// skip the next ucValue RTF characters (the ANSI fallback)
		p.unicodeSkip = top.ucValue

	case "uc":
		top.ucValue = param

	// --- Encoding ---
	case "ansi":
		top.codePage = 1252
	case "mac":
		top.codePage = 10000
	case "pc":
		top.codePage = 437
	case "pca":
		top.codePage = 850
	case "ansicpg":
		if hasParam {
			top.codePage = param
		}

	// --- Special characters ---
	case "emdash":
		p.writeRune('\u2014')
	case "endash":
		p.writeRune('\u2013')
	case "lquote":
		p.writeRune('\u2018')
	case "rquote":
		p.writeRune('\u2019')
	case "ldblquote":
		p.writeRune('\u201c')
	case "rdblquote":
		p.writeRune('\u201d')
	case "bullet":
		p.writeRune('\u2022')
	case "~":
		p.writeRune('\u00a0')
	case "enspace", "emspace", "qmspace":
		p.writeRune(' ')
	case "zwbo", "zwnbo", "zwj", "zwnj":
		// zero-width chars — skip
	case "softline":
		p.writeRune('\n')
	case "softcol", "softpage":
		p.writeParagraph()
	case "cell", "nestcell":
		p.writeRune(' ')
	case "row", "nestrow":
		p.writeRune('\n')
	// Everything else: no text output
	default:
		_ = word
	}
}

// parseHexChar reads \'XX and emits the decoded byte.
func (p *Parser) parseHexChar() error {
	hi, err := p.r.ReadByte()
	if err != nil {
		return err
	}
	lo, err := p.r.ReadByte()
	if err != nil {
		return err
	}

	val, err := strconv.ParseUint(string([]byte{hi, lo}), 16, 8)
	if err != nil {
		return nil // skip malformed
	}

	if p.unicodeSkip > 0 {
		p.unicodeSkip--
		return nil
	}

	// Decode the byte using the current code page
	r := decodeCP(byte(val), p.top().codePage)
	p.writeRune(r)
	return nil
}

// writeChar emits a single ASCII byte as plain text.
func (p *Parser) writeChar(b byte) {
	if p.top().skip {
		return
	}
	if p.unicodeSkip > 0 {
		p.unicodeSkip--
		return
	}
	_ = p.w.WriteByte(b)
}

// writeRune emits a Unicode rune as UTF-8.
func (p *Parser) writeRune(r rune) {
	if p.top().skip {
		return
	}
	_, _ = p.w.WriteRune(r)
}

// writeParagraph emits a paragraph separator.
func (p *Parser) writeParagraph() {
	if p.top().skip {
		return
	}
	_ = p.w.WriteByte('\n')
}

// --- helpers ---

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// ConvertString is a convenience wrapper for small inputs.
func ConvertString(rtf string) (string, error) {
	var buf bytes.Buffer
	p := NewParser(strings.NewReader(rtf), &buf)
	if err := p.Parse(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Convert streams RTF from r, writing plain text to w.
func Convert(r io.Reader, w io.Writer) error {
	return NewParser(r, w).Parse()
}
