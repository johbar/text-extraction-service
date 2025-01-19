/*
Package rtfparser implements a parser for the Rich Text Format.

This code is forked from https://github.com/IntelligenceX/fileconversion,
which itself is forked from https://github.com/J45k4/rtf-go and extracts text from RTF files.

I ported it from standard lib's regexp package to github.com/dlclark/regexp2,
hoping the use of FindNextMatch() instead of FindAllStringSubmatch() might
lower memory requirements when processing large files.
While this seems to be the case the parser still is very inefficient for larger files
(e.g. those containing images.)
*/
package rtfparser

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

type RichTextDoc struct {
	rtfContent string
	metadata   RtfMetadata
}

var destinations = map[string]bool{
	"aftncn":             true,
	"aftnsep":            true,
	"aftnsepc":           true,
	"annotation":         true,
	"atnauthor":          true,
	"atndate":            true,
	"atnicn":             true,
	"atnid":              true,
	"atnparent":          true,
	"atnref":             true,
	"atntime":            true,
	"atrfend":            true,
	"atrfstart":          true,
	"author":             true,
	"background":         true,
	"bkmkend":            true,
	"bkmkstart":          true,
	"blipuid":            true,
	"buptim":             true,
	"category":           true,
	"colorschememapping": true,
	"colortbl":           true,
	"comment":            true,
	"company":            true,
	"creatim":            true,
	"datafield":          true,
	"datastore":          true,
	"defchp":             true,
	"defpap":             true,
	"do":                 true,
	"doccomm":            true,
	"docvar":             true,
	"dptxbxtext":         true,
	"ebcend":             true,
	"ebcstart":           true,
	"factoidname":        true,
	"falt":               true,
	"fchars":             true,
	"ffdeftext":          true,
	"ffentrymcr":         true,
	"ffexitmcr":          true,
	"ffformat":           true,
	"ffhelptext":         true,
	"ffl":                true,
	"ffname":             true,
	"ffstattext":         true,
	"field":              true,
	"file":               true,
	"filetbl":            true,
	"fldinst":            true,
	"fldrslt":            true,
	"fldtype":            true,
	"fname":              true,
	"fontemb":            true,
	"fontfile":           true,
	"fonttbl":            true,
	"footer":             true,
	"footerf":            true,
	"footerl":            true,
	"footerr":            true,
	"footnote":           true,
	"formfield":          true,
	"ftncn":              true,
	"ftnsep":             true,
	"ftnsepc":            true,
	"g":                  true,
	"generator":          true,
	"gridtbl":            true,
	"header":             true,
	"headerf":            true,
	"headerl":            true,
	"headerr":            true,
	"hl":                 true,
	"hlfr":               true,
	"hlinkbase":          true,
	"hlloc":              true,
	"hlsrc":              true,
	"hsv":                true,
	"htmltag":            true,
	"info":               true,
	"keycode":            true,
	"keywords":           true,
	"latentstyles":       true,
	"lchars":             true,
	"levelnumbers":       true,
	"leveltext":          true,
	"lfolevel":           true,
	"linkval":            true,
	"list":               true,
	"listlevel":          true,
	"listname":           true,
	"listoverride":       true,
	"listoverridetable":  true,
	"listpicture":        true,
	"liststylename":      true,
	"listtable":          true,
	// this hides bullet points and enumerations:
	// "listtext":           true,
	"lsdlockedexcept":  true,
	"macc":             true,
	"maccPr":           true,
	"mailmerge":        true,
	"maln":             true,
	"malnScr":          true,
	"manager":          true,
	"margPr":           true,
	"mbar":             true,
	"mbarPr":           true,
	"mbaseJc":          true,
	"mbegChr":          true,
	"mborderBox":       true,
	"mborderBoxPr":     true,
	"mbox":             true,
	"mboxPr":           true,
	"mchr":             true,
	"mcount":           true,
	"mctrlPr":          true,
	"md":               true,
	"mdeg":             true,
	"mdegHide":         true,
	"mden":             true,
	"mdiff":            true,
	"mdPr":             true,
	"me":               true,
	"mendChr":          true,
	"meqArr":           true,
	"meqArrPr":         true,
	"mf":               true,
	"mfName":           true,
	"mfPr":             true,
	"mfunc":            true,
	"mfuncPr":          true,
	"mgroupChr":        true,
	"mgroupChrPr":      true,
	"mgrow":            true,
	"mhideBot":         true,
	"mhideLeft":        true,
	"mhideRight":       true,
	"mhideTop":         true,
	"mhtmltag":         true,
	"mlim":             true,
	"mlimloc":          true,
	"mlimlow":          true,
	"mlimlowPr":        true,
	"mlimupp":          true,
	"mlimuppPr":        true,
	"mm":               true,
	"mmaddfieldname":   true,
	"mmath":            true,
	"mmathPict":        true,
	"mmathPr":          true,
	"mmaxdist":         true,
	"mmc":              true,
	"mmcJc":            true,
	"mmconnectstr":     true,
	"mmconnectstrdata": true,
	"mmcPr":            true,
	"mmcs":             true,
	"mmdatasource":     true,
	"mmheadersource":   true,
	"mmmailsubject":    true,
	"mmodso":           true,
	"mmodsofilter":     true,
	"mmodsofldmpdata":  true,
	"mmodsomappedname": true,
	"mmodsoname":       true,
	"mmodsorecipdata":  true,
	"mmodsosort":       true,
	"mmodsosrc":        true,
	"mmodsotable":      true,
	"mmodsoudl":        true,
	"mmodsoudldata":    true,
	"mmodsouniquetag":  true,
	"mmPr":             true,
	"mmquery":          true,
	"mmr":              true,
	"mnary":            true,
	"mnaryPr":          true,
	"mnoBreak":         true,
	"mnum":             true,
	"mobjDist":         true,
	"moMath":           true,
	"moMathPara":       true,
	"moMathParaPr":     true,
	"mopEmu":           true,
	"mphant":           true,
	"mphantPr":         true,
	"mplcHide":         true,
	"mpos":             true,
	"mr":               true,
	"mrad":             true,
	"mradPr":           true,
	"mrPr":             true,
	"msepChr":          true,
	"mshow":            true,
	"mshp":             true,
	"msPre":            true,
	"msPrePr":          true,
	"msSub":            true,
	"msSubPr":          true,
	"msSubSup":         true,
	"msSubSupPr":       true,
	"msSup":            true,
	"msSupPr":          true,
	"mstrikeBLTR":      true,
	"mstrikeH":         true,
	"mstrikeTLBR":      true,
	"mstrikeV":         true,
	"msub":             true,
	"msubHide":         true,
	"msup":             true,
	"msupHide":         true,
	"mtransp":          true,
	"mtype":            true,
	"mvertJc":          true,
	"mvfmf":            true,
	"mvfml":            true,
	"mvtof":            true,
	"mvtol":            true,
	"mzeroAsc":         true,
	"mzeroDesc":        true,
	"mzeroWid":         true,
	"nesttableprops":   true,
	"nextfile":         true,
	"nonesttables":     true,
	"objalias":         true,
	"objclass":         true,
	"objdata":          true,
	"object":           true,
	"objname":          true,
	"objsect":          true,
	"objtime":          true,
	"oldcprops":        true,
	"oldpprops":        true,
	"oldsprops":        true,
	"oldtprops":        true,
	"oleclsid":         true,
	"operator":         true,
	"panose":           true,
	"password":         true,
	"passwordhash":     true,
	"pgp":              true,
	"pgptbl":           true,
	"picprop":          true,
	"pict":             true,
	"pn":               true,
	"pnseclvl":         true,
	"pntext":           true,
	"pntxta":           true,
	"pntxtb":           true,
	"printim":          true,
	"private":          true,
	"propname":         true,
	"protend":          true,
	"protstart":        true,
	"protusertbl":      true,
	"pxe":              true,
	"result":           true,
	"revtbl":           true,
	"revtim":           true,
	"rsidtbl":          true,
	"rxe":              true,
	"shp":              true,
	"shpgrp":           true,
	"shpinst":          true,
	"shppict":          true,
	"shprslt":          true,
	"shptxt":           true,
	"sn":               true,
	"sp":               true,
	"staticval":        true,
	"stylesheet":       true,
	"subject":          true,
	"sv":               true,
	"svb":              true,
	"tc":               true,
	"template":         true,
	"themedata":        true,
	"title":            true,
	"txe":              true,
	"ud":               true,
	"upr":              true,
	"userprops":        true,
	"wgrffmtfilter":    true,
	"windowcaption":    true,
	"writereservation": true,
	"writereservhash":  true,
	"xe":               true,
	"xform":            true,
	"xmlattrname":      true,
	"xmlattrvalue":     true,
	"xmlclose":         true,
	"xmlname":          true,
	"xmlnstbl":         true,
	"xmlopen":          true,
}

var charmaps = map[string]*charmap.Charmap{
	"437": charmap.CodePage437,
	//	"708":  nil,
	//	"709":  nil,
	//	"710":  nil,
	//	"711":  nil,
	//	"720":  nil,
	//	"819":  nil,
	"850": charmap.CodePage850,
	"852": charmap.CodePage852,
	"860": charmap.CodePage860,
	"862": charmap.CodePage862,
	"863": charmap.CodePage863,
	//	"864":  nil,
	"865": charmap.CodePage865,
	"866": charmap.CodePage866,
	//	"874":  nil,
	//	"932":  nil,
	//	"936":  nil,
	//	"949":  nil,
	//	"950":  nil,
	"1250": charmap.Windows1250,
	"1251": charmap.Windows1251,
	"1252": charmap.Windows1252,
	"1253": charmap.Windows1253,
	"1254": charmap.Windows1254,
	"1255": charmap.Windows1255,
	"1256": charmap.Windows1256,
	"1257": charmap.Windows1257,
	"1258": charmap.Windows1258,
	//	"1361": nil,
}

// special characters to be translated in a way not preserving layout
var charsNoFmt = map[string]string{
	"cell":      " ",
	"par":       " ",
	"sect":      " ",
	"page":      " ",
	"line":      " ",
	"tab":       " ",
	"emdash":    "\u2014",
	"endash":    "\u2013",
	"emspace":   "\u2003",
	"enspace":   "\u2002",
	"qmspace":   "\u2005",
	"bullet":    "\u2022",
	"lquote":    "\u2018",
	"rquote":    "\u2019",
	"ldblquote": "\u201C",
	"rdblquote": "\u201D",
}

// special characters to be translated in a way preserving some layout information
var charsWithFmt = map[string]string{
	"cell":      " | ",
	"row":       "\n",
	"trowd":     "|",
	"par":       "\n",
	"sect":      "\n\n",
	"page":      "\n\n",
	"line":      "\n",
	"tab":       "\t",
	"emdash":    "\u2014",
	"endash":    "\u2013",
	"emspace":   "\u2003",
	"enspace":   "\u2002",
	"qmspace":   "\u2005",
	"bullet":    "\u2022",
	"lquote":    "\u2018",
	"rquote":    "\u2019",
	"ldblquote": "\u201C",
	"rdblquote": "\u201D",
}

var ErrNoRtf error = errors.New("rtfparser: document is not an RTF")

var rtfRegex = regexp2.MustCompile(
	"(?i)"+
		`\\([a-z]{1,32})(-?\d{1,10})?[ ]?`+ //word
		`|\\'([0-9a-f]{2})`+ //arg
		`|\\([^a-z])`+ //hex
		`|([{}])`+ //character
		`|[\r\n]+`+ //brace
		`|(.)`, 0) //tchar

type stackEntry struct {
	NumberOfCharactersToSkip int
	Ignorable                bool
}

func newStackEntry(numberOfCharactersToSkip int, ignorable bool) stackEntry {
	return stackEntry{
		NumberOfCharactersToSkip: numberOfCharactersToSkip,
		Ignorable:                ignorable,
	}
}

func NewFromBytes(data []byte) (d *RichTextDoc, err error) {
	inputRtf := string(data)

	if !IsFileRTF(data) {
		err = ErrNoRtf
		return
	}
	info, err := GetRtfInfo(inputRtf)
	d = &RichTextDoc{rtfContent: inputRtf, metadata: info}
	return
}

func (d *RichTextDoc) Pages() int {
	return -1
}

func (d *RichTextDoc) Data() *[]byte {
	return nil
}

func (d *RichTextDoc) Text(i int) (string, bool) {
	return rtf2text(d.rtfContent, charsWithFmt), false
}

func (d *RichTextDoc) StreamText(w io.Writer) error {
	return rtf2textWriter(d.rtfContent, charsNoFmt, w)
}

// Rtf2SingleLine converts RTF formatted input to
// plain text without preserving any layout and formatting,
// returning one long string
func Rtf2SingleLine(inputRtf string) string {
	return rtf2text(inputRtf, charsNoFmt)
}

// Rtf2Text removes rtf characters from string and returns the new string.
// This function retains some of the layout, e.g. paragraphs/newlines
// tabs and tables.
func Rtf2Text(inputRtf string) string {
	return rtf2text(inputRtf, charsWithFmt)
}

func rtf2text(inputRtf string, specialCharacters map[string]string) string {
	var buf bytes.Buffer
	rtf2textWriter(inputRtf, specialCharacters, &buf)
	return buf.String()
}

func rtf2textWriter(inputRtf string, specialCharacters map[string]string, w io.Writer) error {
	var charMap *charmap.Charmap
	var decoder *encoding.Decoder
	var stack []stackEntry
	var ignorable bool
	ucskip := 1
	curskip := 0

	out := bufio.NewWriter(w)
	match, _ := rtfRegex.FindStringMatch(inputRtf)
	var numMatches uint64 = 0
	for match != nil {
		numMatches++
		word := match.GroupByNumber(1).String()
		arg := match.GroupByNumber(2).String()
		hex := match.GroupByNumber(3).String()
		character := match.GroupByNumber(4).String()
		brace := match.GroupByNumber(5).String()
		tchar := match.GroupByNumber(6).String()

		switch {
		case tchar != "":
			if curskip > 0 {
				curskip--
			} else if !ignorable {
				if charMap == nil || decoder == nil {
					if _, err := out.WriteString(tchar); err != nil {
						return err
					}
				} else {
					tcharDec, err := decoder.String(tchar)
					if err == nil {
						if _, err := out.WriteString(tcharDec); err != nil {
							return err
						}

					}
				}
			}
		case brace != "":
			curskip = 0
			if brace == "{" {
				stack = append(
					stack, newStackEntry(ucskip, ignorable))
			} else if brace == "}" {
				// There was a crash here with item 06ffe2e7-06b6-41d6-9905-3a225fd55537
				// It's fixed by checking l == 0 and handling it as special case
				if l := len(stack); l > 0 {
					entry := stack[l-1]
					stack = stack[:l-1]
					ucskip = entry.NumberOfCharactersToSkip
					ignorable = entry.Ignorable
				}
			}
		case character != "":
			curskip = 0
			if character == "~" {
				if !ignorable {
					if _, err := out.WriteString("\xA0"); err != nil {
						return err
					}
				}
			} else if strings.Contains("{}\\", character) {
				if !ignorable {
					if _, err := out.WriteString(character); err != nil {
						return err
					}
				}
			} else if character == "*" {
				ignorable = true
			}
		case word != "":
			curskip = 0
			if destinations[word] {
				ignorable = true
			} else if ignorable {
			} else if specialCharacters[word] != "" {
				if _, err := out.WriteString(specialCharacters[word]); err != nil {
					return err
				}
			} else if word == "ansicpg" {
				var ok bool
				if charMap, ok = charmaps[arg]; ok {
					decoder = charMap.NewDecoder()
				} else {
					// encoding not supported, continue anyway
				}
			} else if word == "uc" {
				i, _ := strconv.Atoi(arg)
				ucskip = i
			} else if word == "u" {
				c, _ := strconv.Atoi(arg)
				if c < 0 {
					c += 0x10000
				}
				if _, err := out.WriteRune(rune(c)); err != nil {
					return err
				}
				curskip = ucskip
			}
		case hex != "":
			if curskip > 0 {
				curskip--
			} else if !ignorable {
				c, _ := strconv.ParseInt(hex, 16, 0)
				if charMap == nil {
					if _, err := out.WriteRune(rune(c)); err != nil {
						return err
					}
				} else {
					if _, err := out.WriteRune(charMap.DecodeByte(byte(c))); err != nil {
						return err
					}
				}
			}
		}
		match, _ = rtfRegex.FindNextMatch(match)
		out.Flush()
	}
	// log.Printf("Number of matches: %v", numMatches)
	return nil
}

// IsFileRTF checks if the data indicates a RTF file
// RTF has a signature of 7B 5C 72 74 66 31, or in string "{\rtf1"
func IsFileRTF(data []byte) bool {
	return bytes.HasPrefix(data, []byte{0x7B, 0x5C, 0x72, 0x74, 0x66, 0x31})
}

func (d *RichTextDoc) MetadataMap() map[string]string {
	m := make(map[string]string)
	if d.metadata.Author != "" {
		m["x-document-author"] = d.metadata.Author
	}
	if d.metadata.Category != "" {
		m["x-document-category"] = d.metadata.Category
	}
	if d.metadata.Comment != "" {
		m["x-document-comment"] = d.metadata.Comment
	}
	if d.metadata.Company != "" {
		m["x-document-company"] = d.metadata.Company
	}
	if d.metadata.Operator != "" {
		m["x-document-operator"] = d.metadata.Operator
	}

	if d.metadata.Subject != "" {
		m["x-document-subject"] = d.metadata.Subject
	}
	if d.metadata.Title != "" {
		m["x-document-title"] = d.metadata.Title
	}
	if d.metadata.Created != nil {
		m["x-document-created"] = d.metadata.Created.Format(time.RFC3339)
	}
	if d.metadata.Modified != nil {
		m["x-document-modified"] = d.metadata.Modified.Format(time.RFC3339)
	}
	return m
}

// Close is a no-op for RTFs
func (d *RichTextDoc) Close() {
}
