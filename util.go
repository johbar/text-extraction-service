package main

import (
	"strings"
)

// dehyphenateString replaces hyphens at the end of a line
// with the first word from the following line, and removes
// that word from its line.
// taken from https://git.rescribe.xyz/cgit/cgit.cgi/utils/tree/cmd/dehyphenate/main.go
func dehyphenateString(in string) string {
	var newlines []string
	lines := strings.Split(in, "\n")
	for i, line := range lines {
		words := strings.Split(line, " ")
		last := words[len(words)-1]
		// the - 2 here is to account for a trailing newline and counting from zero
		if len(last) > 0 && last[len(last)-1] == '-' && i < len(lines)-2 {
			nextwords := strings.Split(lines[i+1], " ")
			if len(nextwords) > 0 {
				line = line[0:len(line)-1] + nextwords[0]
			}
			if len(nextwords) > 1 {
				lines[i+1] = strings.Join(nextwords[1:], " ")
			} else {
				lines[i+1] = ""
			}
		}
		newlines = append(newlines, line)
	}
	return strings.Join(newlines, " ")
}
