/*
Package dehyphenator implements a simple algorithm for de-hyphenating German text.

	German includes a lot of compounds, some involving hyphens, lowercase and
	uppercase characters.
	This package aims to preserve hyphens when they are part of a compound and to remove
	them at the end of lines whenever they are not.
	Not sure if it is of any use when working with other languages.
*/
package dehyphenator

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"unicode"
)

// Wether to remove all newlines and to replace them with whitespace
var RemoveNewlines bool = false

// Dehyphanate removes newlines and hyphens at the end of lines and
// writes all remaining text back to out. Hyphens are preserved if appropriate.
func Dehyphenate(in io.Reader, out bufio.Writer) error {
	// this is the default, when the trailing char in last line has not been a hyphen
	lastLinesTrailingHyphen := '\x00'
	s := bufio.NewScanner(in)
	defer out.Flush()

	for s.Scan() {
		currentLine := strings.TrimSpace(s.Text())
		if len(currentLine) == 0 || isHyphen(rune(currentLine[0])) {
			// Skip empty and hyphen-only lines
			// if !RemoveNewlines {
			// 	out.WriteRune('\n')
			// }
			continue
		}
		if (lastLinesTrailingHyphen != '\x00') && unicode.IsUpper([]rune(currentLine)[0]) {
			// The last line ended with a hyphen that we removed.
			// The current line starts with an uppercase letter.
			// Now we have to put it back first.
			out.WriteRune(lastLinesTrailingHyphen)
		}
		// reset last line status
		lastLinesTrailingHyphen = '\x00'
		if !endsWithHyphen(currentLine) {
			if _, err := out.WriteString(currentLine); err != nil {
				return err
			}
			if !RemoveNewlines {
				out.WriteRune('\n')
			} else {
				out.WriteRune(' ')
			}
		} else {
			// possible dehyphenation candidate
			if currentRunes := []rune(currentLine); unicode.IsUpper(currentRunes[len(currentRunes)-2]) {
				// But line ends with uppercase rune before hyphen.
				// So keep it as it is.
				if _, err := out.WriteString(currentLine); err != nil {
					return err
				}
			} else {
				// no uppercase rune close to the end of the current line
				// but maybe in next line
				// remove the hyphen and memoize that
				// so we can reattach it in the next iteration if necessary
				lastLinesTrailingHyphen = currentRunes[len(currentRunes)-1]
				if _, err := out.WriteString(string(currentRunes[0 : len(currentRunes)-1])); err != nil {
					return err
				}
			}
		}
		out.Flush()
	}
	return nil
}

func endsWithHyphen(line string) bool {
	r := []rune(line)
	if len(r) < 1 {
		return false
	}
	lastRune := r[len(r)-1]
	return isHyphen(lastRune)
}

func isHyphen(char rune) bool {
	return unicode.Is(unicode.Hyphen, char)
}

// DehyphenateReaderToWriter reads text from in and writes it back to out,
// removing all newlines and hyphens at the end of each line when appropriate.
func DehyphenateReaderToWriter(in io.Reader, out io.Writer) {
	w := bufio.NewWriter(out)
	Dehyphenate(in, *w)
}

func DehyphenateString(in string) (string, error) {
	r := strings.NewReader(in)
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	err := Dehyphenate(r, *w)
	return buf.String(), err
}
