package rtfparser

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Metadata holds document properties extracted from the RTF \info group.
// Fields are empty/zero if not present in the document.
type Metadata struct {

	// Timestamps — nil if absent or unparseable
	Created   *time.Time
	Modified  *time.Time
	Title     string
	Subject   string
	Author    string
	Manager   string
	Company   string
	Operator  string // last saved by
	Category  string
	Keywords  string
	Comment   string
	DocComm   string // document comment (Word "Comments" field)
	HLinkBase string // hyperlink base URL

	// Numeric revision count
	Version int
}

// infoDestinations are the \info sub-destinations that contain plain text.
// The map value is the pointer receiver on Metadata that should be populated.
// We handle this dynamically in metaParser.applyInfoWord instead.
var infoTextDests = map[string]bool{
	"title":     true,
	"subject":   true,
	"author":    true,
	"manager":   true,
	"company":   true,
	"operator":  true,
	"category":  true,
	"keywords":  true,
	"comment":   true,
	"doccomm":   true,
	"hlinkbase": true,
}

// infoTimeDests are the \info sub-destinations that contain an \yr\mo\dy\hr\min\sec
// sequence (or \dttm N on newer writers).
var infoTimeDests = map[string]bool{
	"creatim": true,
	"revtim":  true,
	"printim": true,
	"buptim":  true,
}

// ExtractMetadata reads an RTF stream and returns the document metadata.
// It stops reading as soon as the \info group is fully consumed, making it
// efficient for large documents where metadata always appears near the top.
func ExtractMetadata(r io.Reader) (*Metadata, error) {
	mp := &metaParser{
		r:    bufio.NewReader(r),
		meta: &Metadata{},
	}
	if err := mp.parse(); err != nil {
		return nil, err
	}
	return mp.meta, nil
}

// metaParser is a dedicated streaming parser for the \info group.
type metaParser struct {
	r    *bufio.Reader
	meta *Metadata

	// current info sub-destination (e.g. "author", "title")
	subDest string
	// accumulator for the current sub-destination's text
	textBuf strings.Builder
	// partial time being assembled for a time sub-destination
	pendingTime partialTime

	// group stack depth
	depth int

	// depth at which \info opened (so we know when it closes)
	infoDepth int

	// unicode state
	ucValue     int
	unicodeSkip int

	// code page
	codePage int

	// are we currently inside \info?
	inInfo bool
	// are we inside a time sub-destination?
	inTimeDest bool
}

type partialTime struct {
	yr, mo, dy, hr, min, sec int
}

func (pt partialTime) toTime() time.Time {
	if pt.yr == 0 {
		return time.Time{}
	}
	return time.Date(pt.yr, time.Month(pt.mo), pt.dy, pt.hr, pt.min, pt.sec, 0, time.UTC)
}

func (mp *metaParser) parse() error {
	mp.ucValue = 1
	mp.codePage = 1252

	for {
		b, err := mp.r.ReadByte()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("rtfparse/meta: read error: %w", err)
		}

		switch b {
		case '{':
			mp.depth++
		case '}':
			if mp.inInfo {
				if mp.depth == mp.infoDepth {
					// Closing the \info group itself — flush and stop.
					mp.flushSubDest()
					return nil
				}
				if mp.subDest != "" && mp.depth == mp.infoDepth+1 {
					// Closing a sub-destination group.
					mp.flushSubDest()
				}
			}
			mp.depth--
		case '\\':
			if err := mp.parseControl(); err != nil {
				return err
			}
		case '\r', '\n':
			// ignored
		default:
			mp.accumulate(b)
		}
	}
	return nil
}

func (mp *metaParser) parseControl() error {
	b, err := mp.r.ReadByte()
	if err != nil {
		return fmt.Errorf("rtfparse/meta: unexpected EOF in control: %w", err)
	}

	switch {
	case b == '\'':
		return mp.parseHexChar()
	case b == '*':
		// ignorable destination marker — we just let it pass; sub-dests we
		// don't recognise will simply accumulate into an empty subDest and
		// be discarded on flush.
		return nil
	case b == '\\', b == '{', b == '}':
		mp.accumulateRune(rune(b))
		return nil
	case b == '\r', b == '\n':
		return nil
	case b == '~':
		mp.accumulateRune('\u00a0')
		return nil
	case b == '_':
		mp.accumulateRune('\u2011')
		return nil
	case b == '-':
		return nil
	case isLetter(b):
		return mp.parseWord(b)
	default:
		return nil
	}
}

func (mp *metaParser) parseWord(first byte) error {
	var buf [64]byte
	buf[0] = first
	n := 1

	for {
		b, err := mp.r.ReadByte()
		if err != nil {
			break
		}
		if isLetter(b) {
			if n < len(buf) {
				buf[n] = b
				n++
			}
		} else {
			if b == '-' || isDigit(b) {
				return mp.parseWordWithParam(string(buf[:n]), b)
			}
			if b != ' ' {
				_ = mp.r.UnreadByte()
			}
			break
		}
	}

	mp.applyWord(string(buf[:n]), 0, false)
	return nil
}

func (mp *metaParser) parseWordWithParam(word string, sign byte) error {
	negative := sign == '-'
	var digits [20]byte
	n := 0
	if isDigit(sign) {
		digits[0] = sign
		n = 1
	}

	for {
		b, err := mp.r.ReadByte()
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
				_ = mp.r.UnreadByte()
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

	mp.applyWord(word, param, true)
	return nil
}

func (mp *metaParser) applyWord(word string, param int, hasParam bool) {
	// Encoding words that can appear before \info
	switch word {
	case "ansi":
		mp.codePage = 1252
	case "mac":
		mp.codePage = 10000
	case "pc":
		mp.codePage = 437
	case "pca":
		mp.codePage = 850
	case "ansicpg":
		if hasParam {
			mp.codePage = param
		}
	case "uc":
		mp.ucValue = param
	case "u":
		r := rune(param)
		if r < 0 {
			r += 65536
		}
		mp.accumulateRune(r)
		mp.unicodeSkip = mp.ucValue
		return
	}

	if !mp.inInfo {
		// Only care about entering \info
		if word == "info" {
			mp.inInfo = true
			mp.infoDepth = mp.depth
		}
		return
	}

	// Inside \info — watch for sub-destinations and time fields
	if infoTextDests[word] {
		mp.flushSubDest()
		mp.subDest = word
		mp.inTimeDest = false
		mp.textBuf.Reset()
		return
	}

	if infoTimeDests[word] {
		mp.flushSubDest()
		mp.subDest = word
		mp.inTimeDest = true
		mp.pendingTime = partialTime{}
		return
	}

	// Numeric time components
	if mp.inTimeDest && hasParam {
		switch word {
		case "yr":
			mp.pendingTime.yr = param
		case "mo":
			mp.pendingTime.mo = param
		case "dy":
			mp.pendingTime.dy = param
		case "hr":
			mp.pendingTime.hr = param
		case "min":
			mp.pendingTime.min = param
		case "sec":
			mp.pendingTime.sec = param
		}
		return
	}

	// \version N
	if word == "version" && hasParam {
		mp.meta.Version = param
		return
	}

	// Special characters inside text sub-destinations
	if mp.subDest != "" && !mp.inTimeDest {
		switch word {
		case "emdash":
			mp.accumulateRune('\u2014')
		case "endash":
			mp.accumulateRune('\u2013')
		case "lquote":
			mp.accumulateRune('\u2018')
		case "rquote":
			mp.accumulateRune('\u2019')
		case "ldblquote":
			mp.accumulateRune('\u201c')
		case "rdblquote":
			mp.accumulateRune('\u201d')
		case "tab":
			mp.accumulateRune('\t')
		case "enspace", "emspace", "qmspace":
			mp.accumulateRune(' ')
		}
	}
}

// flushSubDest writes the accumulated text (or time) into the Metadata struct.
func (mp *metaParser) flushSubDest() {
	if mp.subDest == "" {
		return
	}

	if mp.inTimeDest {
		t := mp.pendingTime.toTime()
		switch mp.subDest {
		case "creatim":
			mp.meta.Created = &t
		case "revtim":
			mp.meta.Modified = &t
		}
	} else {
		text := strings.TrimSpace(mp.textBuf.String())
		switch mp.subDest {
		case "title":
			mp.meta.Title = text
		case "subject":
			mp.meta.Subject = text
		case "author":
			mp.meta.Author = text
		case "manager":
			mp.meta.Manager = text
		case "company":
			mp.meta.Company = text
		case "operator":
			mp.meta.Operator = text
		case "category":
			mp.meta.Category = text
		case "keywords":
			mp.meta.Keywords = text
		case "comment":
			mp.meta.Comment = text
		case "doccomm":
			mp.meta.DocComm = text
		case "hlinkbase":
			mp.meta.HLinkBase = text
		}
	}

	mp.subDest = ""
	mp.inTimeDest = false
	mp.textBuf.Reset()
}

// accumulate adds a raw byte to the current sub-destination buffer, respecting
// unicode skip state and the current group's skip flag.
func (mp *metaParser) accumulate(b byte) {
	if mp.subDest == "" || mp.inTimeDest {
		return
	}
	if mp.unicodeSkip > 0 {
		mp.unicodeSkip--
		return
	}
	r := decodeCP(b, mp.codePage)
	mp.textBuf.WriteRune(r)
}

func (mp *metaParser) accumulateRune(r rune) {
	if mp.subDest == "" || mp.inTimeDest {
		return
	}
	mp.textBuf.WriteRune(r)
}

func (mp *metaParser) parseHexChar() error {
	hi, err := mp.r.ReadByte()
	if err != nil {
		return err
	}
	lo, err := mp.r.ReadByte()
	if err != nil {
		return err
	}
	val, err := strconv.ParseUint(string([]byte{hi, lo}), 16, 8)
	if err != nil {
		return nil
	}
	if mp.unicodeSkip > 0 {
		mp.unicodeSkip--
		return nil
	}
	r := decodeCP(byte(val), mp.codePage)
	mp.accumulateRune(r)
	return nil
}
