package officexmlparser

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"slices"
)

var excessiveWhitespace regexp.Regexp = *regexp.MustCompile(`\s{2,}`)

// XmlToText reads XML data from r and writes its text (char data) to w.
// It preserves newlines, but removes duplicate whitespace. Text before the startsWith Tag is ignored.
func XmlToText(r io.Reader, w io.Writer, startWith string, breakElements []string) error {
	d := xml.NewDecoder(r)
	var err error
	var token xml.Token

	// We skip everything up to the start token, usually the body tag
	for {
		token, err = d.RawToken()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if t, ok := token.(xml.StartElement); ok {
			if t.Name.Local == startWith {
				break
			}
		}
	}

	for err == nil {
		token, err = d.RawToken()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		switch t := token.(type) {
		case xml.CharData:
			orgLength := len(t)
			cleaned := excessiveWhitespace.ReplaceAll(t, []byte{' '})

			// if what remains is not just a single whitespace
			// and it wasn't before cleaning,
			// write it to w
			if orgLength == 1 || !bytes.Equal(cleaned, []byte{' '}) {
				if _, err := w.Write(cleaned); err != nil {
					return err
				}
			}
		case xml.StartElement:
			if t.Name.Local == "tableStyleId" {
				// the next CDATA in pptx is an UUID, not to be printed
				// so we just skip it
				if _, err := d.RawToken(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if slices.Contains(breakElements, t.Name.Local) {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					return err
				}
			}

			// in ODP this is a space tag
			if t.Name.Space == "text" && t.Name.Local == "s" {
				if _, err := w.Write([]byte{' '}); err != nil {
					return err
				}
			}
		}
	}
	return err
}
