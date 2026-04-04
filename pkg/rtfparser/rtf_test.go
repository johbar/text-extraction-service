package rtfparser

import (
	"strings"
	"testing"
)

type testCase struct {
	name string
	rtf  string
	want string
}

var tests = []testCase{
	{
		name: "plain text",
		rtf:  `{\rtf1\ansi Hello World}`,
		want: "Hello World",
	},
	{
		name: "bold ignored, text preserved",
		rtf:  `{\rtf1\ansi \b Bold\b0  normal}`,
		want: "Bold normal",
	},
	{
		name: "paragraph break",
		rtf:  `{\rtf1\ansi Line one\par Line two}`,
		want: "Line one\nLine two",
	},
	{
		name: "multiple paragraphs",
		rtf:  `{\rtf1\ansi First\par Second\par Third}`,
		want: "First\nSecond\nThird",
	},
	{
		name: "tab character",
		rtf:  `{\rtf1\ansi Col1\tab Col2}`,
		want: "Col1\tCol2",
	},
	{
		name: "em dash",
		rtf:  `{\rtf1\ansi before\emdash after}`,
		want: "before\u2014after",
	},
	{
		name: "en dash",
		rtf:  `{\rtf1\ansi before\endash after}`,
		want: "before\u2013after",
	},
	{
		name: "smart quotes",
		rtf:  `{\rtf1\ansi \ldblquote hello\rdblquote}`,
		want: "\u201chello\u201d",
	},
	{
		name: "bullet point",
		rtf:  `{\rtf1\ansi \bullet item}`,
		want: "\u2022item",
	},
	{
		name: "hex escape CP1252 euro sign",
		rtf:  "{\\rtf1\\ansi\\ansicpg1252 \\'80}",
		want: "\u20AC", // €
	},
	{
		name: "hex escape accented char",
		rtf:  "{\\rtf1\\ansi\\ansicpg1252 caf\\'e9}",
		want: "caf\u00e9", // café
	},
	{
		name: "unicode control word",
		rtf:  `{\rtf1\ansi \u8364?}`,
		want: "\u20AC",
	},
	{
		name: "unicode negative value (signed 16-bit)",
		rtf:  `{\rtf1\ansi \u-32768?}`,
		want: "\u8000", // -32768 + 65536 = 32768 = 0x8000
	},
	{
		name: "skip fonttbl",
		rtf:  `{\rtf1\ansi {\fonttbl{\f0 Arial;}}Hello}`,
		want: "Hello",
	},
	{
		name: "skip colortbl",
		rtf:  `{\rtf1\ansi {\colortbl;\red0\green0\blue0;}Hello}`,
		want: "Hello",
	},
	{
		name: "skip info",
		rtf:  `{\rtf1\ansi {\info{\author Joe}}Hello}`,
		want: "Hello",
	},
	{
		name: "skip pict",
		rtf:  `{\rtf1\ansi {\pict\wmetafile8 AABBCC}Hello}`,
		want: "Hello",
	},
	{
		name: "field result included",
		rtf:  `{\rtf1\ansi {\field{\fldinst HYPERLINK "http://x.com"}{\fldrslt Click here}}}`,
		want: "Click here",
	},
	{
		name: "ignorable destination star",
		rtf:  `{\rtf1\ansi {\*\customdest secret}visible}`,
		want: "visible",
	},
	{
		name: "nested groups",
		rtf:  `{\rtf1\ansi outer {\b bold} outer}`,
		want: "outer bold outer",
	},
	{
		name: "escaped braces",
		rtf:  `{\rtf1\ansi \{brace\}}`,
		want: "{brace}",
	},
	{
		name: "line break",
		rtf:  `{\rtf1\ansi line1\line line2}`,
		want: "line1\nline2",
	},
	{
		name: "non-breaking space",
		rtf:  `{\rtf1\ansi hello\~world}`,
		want: "hello\u00a0world",
	},
	{
		name: "skip stylesheet",
		rtf:  `{\rtf1\ansi {\stylesheet{\s0 Normal;}}Text}`,
		want: "Text",
	},
	{
		name: "complex document",
		rtf: "{\\rtf1\\ansi\\ansicpg1252\\deff0" +
			"{\\fonttbl{\\f0\\froman\\fcharset0 Times New Roman;}}" +
			"{\\colortbl ;\\red0\\green0\\blue0;}" +
			"\\widowctrl\\wpaper12240\\wpapr15840\\margl1800\\margr1800\\margt1440\\margb1440" +
			"\\f0\\fs24 " +
			"This is {\\b bold} and {\\i italic} text.\\par " +
			"Second paragraph with caf\\'e9 and \\emdash dash.\\par " +
			"}",
		want: "This is bold and italic text.\nSecond paragraph with caf\u00e9 and \u2014dash.\n",
	},
	{
		name: "uc2 unicode skip",
		rtf:  `{\rtf1\ansi\uc2 \u955??}`,
		want: "\u03BB", // lambda, skipping 2 '?' chars
	},
}

func TestParse(t *testing.T) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ConvertString(tc.rtf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("\nRTF:  %s\ngot:  %q\nwant: %q", tc.rtf, got, tc.want)
			}
		})
	}
}

func TestConvert_LargeStream(t *testing.T) {
	// Build a large RTF with 10,000 paragraphs to verify streaming works without OOM
	var sb strings.Builder
	sb.WriteString(`{\rtf1\ansi `)
	for i := 0; i < 10_000; i++ {
		sb.WriteString(`Line of text goes here\par `)
	}
	sb.WriteString(`}`)

	rtf := sb.String()
	var out strings.Builder
	err := Convert(strings.NewReader(rtf), &out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Line of text goes here") {
		t.Error("output missing expected content")
	}
}

func TestConvert_EmptyInput(t *testing.T) {
	got, err := ConvertString(`{\rtf1\ansi }`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" && got != " " {
		t.Errorf("expected empty-ish, got %q", got)
	}
}

func BenchmarkParse(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`{\rtf1\ansi\ansicpg1252 `)
	for i := 0; i < 1000; i++ {
		sb.WriteString(`Hello {\b World}\par `)
	}
	sb.WriteString(`}`)
	rtf := sb.String()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ConvertString(rtf)
	}
}