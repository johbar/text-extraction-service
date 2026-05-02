package dehyphenator

import (
	"io"
	"strings"
	"testing"
)

// run feeds input in one Write call and checks the output.
func run(t *testing.T, input, want string) {
	t.Helper()
	var buf strings.Builder
	w := New(&buf, false)
	if _, err := io.Copy(w, strings.NewReader(input)); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("\ninput: %q\n  got: %q\n want: %q", input, got, want)
	}
}

// runChunked feeds the same input byte-by-byte (worst-case chunking) and
// expects the same output, verifying that the streaming path is correct.
func runChunked(t *testing.T, input, want string) {
	t.Helper()
	var buf strings.Builder
	w := New(&buf, false)
	for i := 0; i < len(input); i++ {
		if _, err := w.Write([]byte{input[i]}); err != nil {
			t.Fatalf("Write error at byte %d: %v", i, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
	if got := buf.String(); got != want {
		t.Errorf("chunked\ninput: %q\n  got: %q\n want: %q", input, got, want)
	}
}

// check runs both the whole-input and byte-by-byte paths.
func check(t *testing.T, input, want string) {
	t.Helper()
	run(t, input, want)
	runChunked(t, input, want)
}

// ── Core dehyphenation rules ──────────────────────────────────────────────────

func TestPlainLine(t *testing.T) {
	// Lines without trailing hyphens pass through unchanged (modulo separator).
	check(t, "Hallo Welt\n", "Hallo Welt\n")
}

func TestLineBreakHyphenRemoved(t *testing.T) {
	// Lowercase before the hyphen, lowercase after → line-break hyphen, remove it.
	check(t,
		"Stra-\nße\n",
		"Straße\n",
	)
}

func TestLineBreakHyphenRestoredBeforeUppercase(t *testing.T) {
	// Lowercase before the hyphen, uppercase after → compound, restore it.
	check(t,
		"EU-\nInstitution\n",
		"EU-Institution\n",
	)
}

func TestAbbreviationCompoundKept(t *testing.T) {
	// Uppercase immediately before the hyphen: keep hyphen,
	// join lines without a separator.
	check(t,
		"E-\nMail\n",
		"E-Mail\n",
	)
}

func TestMultipleLinesNoHyphens(t *testing.T) {
	check(t,
		"Erste Zeile\nZweite Zeile\nDritte Zeile\n",
		"Erste Zeile\nZweite Zeile\nDritte Zeile\n",
	)
}

func TestMixedHyphensInDocument(t *testing.T) {
	input := strings.Join([]string{
		"Das ist ein Bei-",   // line-break hyphen, next starts lowercase → remove
		"spiel für die",      // continues previous
		"EU-",               // line-break hyphen, next starts uppercase → restore
		"Kommission und E-",   // uppercase before hyphen → abbreviation compound
		"Mail-Adressen.\n",   // end
	}, "\n")
	// "spiel für die" has no trailing hyphen → gets a newline separator.
	want := "Das ist ein Beispiel für die\nEU-Kommission und E-Mail-Adressen.\n"
	check(t, input, want)
}

// ── Empty and degenerate lines ────────────────────────────────────────────────

func TestEmptyLinesSkipped(t *testing.T) {
	check(t,
		"Wort\n\nNoch\n",
		"Wort\nNoch\n",
	)
}

func TestHyphenOnlyLineSkipped(t *testing.T) {
	check(t, "-\nWort\n", "Wort\n")
}

func TestLastHyphenSurvivesBlankLine(t *testing.T) {
	// A blank line between the hyphenated part and its continuation must not
	// discard the pending hyphen.
	check(t,
		"wei-\n\nter\n",
		"weiter\n",
	)
}

// ── Leading and trailing whitespace ──────────────────────────────────────────

func TestLeadingWhitespaceTrimmed(t *testing.T) {
	check(t, "   Einrückung\n", "Einrückung\n")
}

func TestTrailingWhitespaceTrimmed(t *testing.T) {
	check(t, "Wort   \n", "Wort\n")
}

func TestLeadingWhitespaceBeforeHyphenLine(t *testing.T) {
	check(t,
		"   wei-\n   ter\n",
		"weiter\n",
	)
}

// ── RemoveNewlines mode ───────────────────────────────────────────────────────

func TestRemoveNewlines(t *testing.T) {
	var buf strings.Builder
	w := New(&buf, true)
	_, _ = w.Write([]byte("Erste Zeile\nZweite Zeile\n"))
	_ = w.Close()
	want := "Erste Zeile Zweite Zeile "
	if got := buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRemoveNewlinesWithHyphen(t *testing.T) {
	var buf strings.Builder
	w := New(&buf, true)
	_, _ = w.Write([]byte("wei-\nter\n"))
	_ = w.Close()
	// Hyphen removed, space written as separator after the joined word.
	if got := buf.String(); got != "weiter " {
		t.Errorf("got %q, want %q", got, "weiter ")
	}
}

// ── No trailing newline (Close flushes) ───────────────────────────────────────

func TestNoTrailingNewline(t *testing.T) {
	// Close flushes remaining content and writes a separator, just like a
	// normal line — consistent with the original scanner-based behaviour.
	check(t, "Wort", "Wort\n")
}

func TestNoTrailingNewlineAfterHyphen(t *testing.T) {
	// "Wort-" strips the hyphen; "Teil" starts with uppercase so it is
	// restored, giving "Wort-Teil" plus a separator from Close.
	check(t, "Wort-\nTeil", "Wort-Teil\n")
}

// ── Chunk-boundary edge cases ─────────────────────────────────────────────────

func TestChunkBreakBeforeNewline(t *testing.T) {
	// The '\n' arrives in a separate Write call from the content.
	var buf strings.Builder
	w := New(&buf, false)
	_, _ = w.Write([]byte("Hallo Welt"))
	_, _ = w.Write([]byte("\n"))
	_ = w.Close()
	if got := buf.String(); got != "Hallo Welt\n" {
		t.Errorf("got %q", got)
	}
}

func TestChunkBreakInsideAbbreviationHyphen(t *testing.T) {
	// "E" and "-\nMail\n" arrive in separate Write calls.
	// prevContentRune must bridge the gap so the uppercase check still works.
	var buf strings.Builder
	w := New(&buf, false)
	_, _ = w.Write([]byte("E"))
	_, _ = w.Write([]byte("-\nMail\n"))
	_ = w.Close()
	if got := buf.String(); got != "E-Mail\n" {
		t.Errorf("got %q, want %q", got, "E-Mail\n")
	}
}

func TestChunkBreakInsideMultibyteRune(t *testing.T) {
	// 'ß' is two bytes (0xC3 0x9F). Split the input right through it.
	word := "Straße\n"
	bs := []byte(word)
	ssIdx := strings.Index(word, "ß")

	var buf strings.Builder
	w := New(&buf, false)
	_, _ = w.Write(bs[:ssIdx+1]) // first byte of 'ß'
	_, _ = w.Write(bs[ssIdx+1:]) // second byte of 'ß' + rest
	_ = w.Close()
	if got := buf.String(); got != word {
		t.Errorf("got %q, want %q", got, word)
	}
}
