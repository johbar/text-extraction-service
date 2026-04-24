package docparser

// metadata.go — OLE Property Set extraction for Word Binary (.doc) files.
//
// Reads \005SummaryInformation and \005DocumentSummaryInformation streams
// from the CFB container and decodes them into a Metadata struct.
//
// String encoding
// ---------------
// OLE Property Sets contain two string variant types:
//
//   VT_LPWSTR (0x001F) — UTF-16LE, length in code units.  Always correct.
//   VT_LPSTR  (0x001E) — Code-page-encoded bytes.  The property set carries
//                        a CodePage property (PID 0x0001, VT_I2) that names
//                        the encoding.  We handle:
//                          1200  → UTF-16LE  (common in WPS/modern writers)
//                          65001 → UTF-8
//                          other → Windows-1252 via w1252Rune (correct for
//                                  the vast majority of Western documents)
//
// References
// ----------
//   [MS-OLEPS] §2.2  PropertySetHeader
//   [MS-OLEPS] §2.3  PropertyIdentifierAndOffset
//   [MS-OLEPS] §2.14 SummaryInformation Property Set
//   [MS-OLEPS] §2.15 DocumentSummaryInformation Property Set

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"
)

// Metadata holds document properties extracted from the two OLE property
// streams that Word Binary files always carry.
type Metadata struct {
	// From \005SummaryInformation
	Title          string
	Subject        string
	Author         string
	Keywords       string
	Comments       string
	Template       string
	LastAuthor     string
	RevisionNumber string
	Application    string
	Created        time.Time
	LastSaved      time.Time
	LastPrinted    time.Time
	PageCount      int32
	WordCount      int32
	CharCount      int32
	Security       int32

	// From \005DocumentSummaryInformation
	Category      string
	Manager       string
	Company       string
	CharCountFull int32 // chars including spaces (pid 0x0010 in DocSummary)
}

// ── Property Set parsing ──────────────────────────────────────────────────────

type propVal struct {
	vt   uint16
	data []byte // raw bytes immediately after the 4-byte type indicator
}

// parsePropertySet parses a single-section OLE property set stream.
// It returns the declared code page (PID 0x0001) and a map of pid → propVal.
// [MS-OLEPS] §2.2, §2.3
func parsePropertySet(raw []byte) (codePage uint16, props map[uint32]propVal, err error) {
	props = make(map[uint32]propVal)

	// Minimum size: 28-byte stream header + 20-byte section entry
	if len(raw) < 48 {
		return 0, nil, fmt.Errorf("stream too short (%d bytes)", len(raw))
	}

	byteOrder := binary.LittleEndian.Uint16(raw[0:])
	if byteOrder != 0xFFFE {
		return 0, nil, fmt.Errorf("unexpected byte order mark 0x%04X", byteOrder)
	}

	numSets := binary.LittleEndian.Uint32(raw[24:])
	if numSets == 0 {
		return 0, props, nil
	}

	// Stream header layout [MS-OLEPS §2.2]:
	//   [0 ] ByteOrder        2
	//   [2 ] Version          2
	//   [4 ] SystemIdentifier 4
	//   [8 ] CLSID           16
	//   [24] NumPropertySets  4
	//   [28] FMTID0          16  ← first section's FMTID
	//   [44] Offset0          4  ← first section's byte offset within stream
	setOffset := int(binary.LittleEndian.Uint32(raw[44:]))

	if len(raw) < setOffset+8 {
		return 0, nil, fmt.Errorf("section offset %d out of range", setOffset)
	}

	// Section header [MS-OLEPS §2.3]: Size(4) + NumProperties(4)
	numProps := int(binary.LittleEndian.Uint32(raw[setOffset+4:]))

	// PropertyIdentifierAndOffset array: NumProperties × 8 bytes (pid + offset)
	pairBase := setOffset + 8
	if len(raw) < pairBase+numProps*8 {
		return 0, nil, fmt.Errorf("property pairs extend beyond stream")
	}

	for i := 0; i < numProps; i++ {
		pid := binary.LittleEndian.Uint32(raw[pairBase+i*8:])
		poff := int(binary.LittleEndian.Uint32(raw[pairBase+i*8+4:]))
		abs := setOffset + poff

		// Property dictionary (pid 0) and locale (pid 0x80000000) are not
		// typed values; skip them.
		if pid == 0x00000000 || pid == 0x80000000 {
			continue
		}

		if len(raw) < abs+4 {
			continue // truncated property, skip
		}
		vt := binary.LittleEndian.Uint16(raw[abs:])
		// Bytes [2:4] of the type indicator are reserved padding; skip.
		valueStart := abs + 4

		if valueStart > len(raw) {
			continue
		}
		// Cap the slice to avoid excess allocation; decoders only read what they need.
		end := len(raw)
		if end > valueStart+4096 {
			end = valueStart + 4096
		}

		pv := propVal{vt: vt, data: raw[valueStart:end]}
		props[pid] = pv

		// Capture the code page declaration (PID 0x0001, VT_I2) as soon as
		// we see it so subsequent VT_LPSTR values can be decoded correctly.
		if pid == 0x0001 && vt == 0x0002 && len(pv.data) >= 2 {
			codePage = binary.LittleEndian.Uint16(pv.data[:2])
		}
	}
	return codePage, props, nil
}

// ── SummaryInformation (FMTID: F29F85E0-4FF9-1068-AB91-08002B27B3D9) ────────
//
// [MS-OLEPS] §2.14 property IDs:
//   0x0002  Title           0x0003  Subject         0x0004  Author
//   0x0005  Keywords        0x0006  Comments        0x0007  Template
//   0x0008  LastAuthor      0x0009  RevisionNumber  0x000A  LastPrinted
//   0x000C  Created         0x000D  LastSaved       0x000E  PageCount
//   0x000F  WordCount       0x0010  CharCount       0x0012  AppName
//   0x0013  Security

func parseSummaryInformation(raw []byte, m *Metadata) error {
	codePage, props, err := parsePropertySet(raw)
	if err != nil {
		return err
	}
	str := func(pid uint32) string { return propString(props, pid, codePage) }
	m.Title = str(0x0002)
	m.Subject = str(0x0003)
	m.Author = str(0x0004)
	m.Keywords = str(0x0005)
	m.Comments = str(0x0006)
	m.Template = str(0x0007)
	m.LastAuthor = str(0x0008)
	m.RevisionNumber = str(0x0009)
	m.Application = str(0x0012)
	m.LastPrinted = propFileTime(props, 0x000A)
	m.Created = propFileTime(props, 0x000C)
	m.LastSaved = propFileTime(props, 0x000D)
	m.PageCount = propI4(props, 0x000E)
	m.WordCount = propI4(props, 0x000F)
	m.CharCount = propI4(props, 0x0010)
	m.Security = propI4(props, 0x0013)
	return nil
}

// ── DocumentSummaryInformation (FMTID: D5CDD502-2E9C-101B-9397-08002B2CF9AE) ─
//
// [MS-OLEPS] §2.15 property IDs used here:
//   0x000D  Category   0x000E  Manager   0x000F  Company
//   0x0010  CharCountFull (characters including spaces)

func parseDocumentSummaryInformation(raw []byte, m *Metadata) error {
	codePage, props, err := parsePropertySet(raw)
	if err != nil {
		return err
	}
	str := func(pid uint32) string { return propString(props, pid, codePage) }
	m.Category = str(0x000D)
	m.Manager = str(0x000E)
	m.Company = str(0x000F)
	m.CharCountFull = propI4(props, 0x0010)
	return nil
}

// ── Value decoders ────────────────────────────────────────────────────────────

// propString returns the string value of a property, or "" if absent or wrong type.
func propString(props map[uint32]propVal, pid uint32, codePage uint16) string {
	pv, ok := props[pid]
	if !ok {
		return ""
	}
	switch pv.vt {
	case 0x001E: // VT_LPSTR — code-page-encoded, null-terminated, length-prefixed
		return decodeLPSTR(pv.data, codePage)
	case 0x001F: // VT_LPWSTR — UTF-16LE, null-terminated, length in code units
		return decodeLPWSTR(pv.data)
	}
	return ""
}

// propI4 returns the int32 value of a VT_I4 or VT_UI4 property, or 0.
func propI4(props map[uint32]propVal, pid uint32) int32 {
	pv, ok := props[pid]
	if !ok || (pv.vt != 0x0003 && pv.vt != 0x0013) || len(pv.data) < 4 {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(pv.data[:4]))
}

// propFileTime returns the time.Time value of a VT_FILETIME property, or zero.
func propFileTime(props map[uint32]propVal, pid uint32) time.Time {
	pv, ok := props[pid]
	if !ok || pv.vt != 0x0040 || len(pv.data) < 8 {
		return time.Time{}
	}
	lo := binary.LittleEndian.Uint32(pv.data[0:4])
	hi := binary.LittleEndian.Uint32(pv.data[4:8])
	ft := (uint64(hi) << 32) | uint64(lo)
	if ft == 0 {
		return time.Time{}
	}
	// Windows FILETIME: 100-nanosecond intervals since 1601-01-01 UTC.
	const epochDiff = uint64(116444736000000000)
	if ft < epochDiff {
		return time.Time{}
	}
	ticks := ft - epochDiff
	return time.Unix(int64(ticks/10000000), int64((ticks%10000000)*100)).UTC()
}

// ── String decoders ───────────────────────────────────────────────────────────

// decodeLPWSTR decodes a VT_LPWSTR value.
// Wire format: Count(4 bytes) + Count×uint16 (UTF-16LE, null-terminated).
func decodeLPWSTR(data []byte) string {
	if len(data) < 4 {
		return ""
	}
	count := int(binary.LittleEndian.Uint32(data[:4]))
	if count == 0 || len(data) < 4+count*2 {
		return ""
	}
	u16 := make([]uint16, count)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(data[4+i*2:])
	}
	s := string(utf16.Decode(u16))
	if idx := strings.IndexByte(s, 0); idx >= 0 {
		s = s[:idx]
	}
	return s
}

// decodeLPSTR decodes a VT_LPSTR value using the property set's code page.
// Wire format: Count(4 bytes) + Count bytes (code-page-encoded, null-terminated).
//
// Code page handling:
//
//	1200  → UTF-16LE  (non-standard but written by some modern tools)
//	65001 → UTF-8
//	other → Windows-1252 via w1252Rune (covers virtually all Western docs;
//	        for CJK or other scripts, replace with golang.org/x/text/encoding)
func decodeLPSTR(data []byte, codePage uint16) string {
	if len(data) < 4 {
		return ""
	}
	count := int(binary.LittleEndian.Uint32(data[:4]))
	if count == 0 || len(data) < 4+count {
		return ""
	}
	raw := data[4 : 4+count]

	switch codePage {
	case 1200: // UTF-16LE packed into a VT_LPSTR (non-standard)
		if len(raw)%2 != 0 {
			raw = raw[:len(raw)-1]
		}
		u16 := make([]uint16, len(raw)/2)
		for i := range u16 {
			u16[i] = binary.LittleEndian.Uint16(raw[i*2:])
		}
		s := string(utf16.Decode(u16))
		if idx := strings.IndexByte(s, 0); idx >= 0 {
			s = s[:idx]
		}
		return s

	case 65001: // UTF-8
		s := string(raw)
		if idx := strings.IndexByte(s, 0); idx >= 0 {
			s = s[:idx]
		}
		return s

	default:
		// Windows-1252: correct for the overwhelming majority of Western
		// documents. Our w1252Rune table is defined in doc2text.go.
		var sb strings.Builder
		sb.Grow(len(raw))
		for _, b := range raw {
			if b == 0 {
				break // null terminator
			}
			if r := w1252Rune(b); r != 0 {
				sb.WriteRune(r)
			}
		}
		return sb.String()
	}
}
