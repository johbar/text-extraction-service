package rtfparser

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestExtractMetadata_Basic(t *testing.T) {
	rtf := `{\rtf1\ansi\ansicpg1252
{\info
{\title My Document}
{\subject Testing}
{\author Jane Doe}
{\company Acme Corp}
{\keywords go rtf parser}
{\comment A simple test document}
}
Some body text here.
}`

	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct{ got, want, field string }{
		{meta.Title, "My Document", "Title"},
		{meta.Subject, "Testing", "Subject"},
		{meta.Author, "Jane Doe", "Author"},
		{meta.Company, "Acme Corp", "Company"},
		{meta.Keywords, "go rtf parser", "Keywords"},
		{meta.Comment, "A simple test document", "Comment"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.field, c.got, c.want)
		}
	}
}

func TestExtractMetadata_Timestamps(t *testing.T) {
	rtf := `{\rtf1\ansi
{\info
{\creatim\yr2023\mo6\dy15\hr9\min30\sec0}
{\revtim\yr2024\mo1\dy2\hr14\min0\sec45}
{\printim\yr2024\mo3\dy10\hr8\min0\sec0}
}
}`

	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantCreated := time.Date(2023, 6, 15, 9, 30, 0, 0, time.UTC)
	wantSaved := time.Date(2024, 1, 2, 14, 0, 45, 0, time.UTC)

	if !meta.Created.Equal(wantCreated) {
		t.Errorf("CreatedAt: got %v, want %v", meta.Created, wantCreated)
	}
	if !meta.Modified.Equal(wantSaved) {
		t.Errorf("SavedAt: got %v, want %v", meta.Modified, wantSaved)
	}

}

func TestExtractMetadata_Version(t *testing.T) {
	rtf := `{\rtf1\ansi{\info\version14{\author Bob}}}body`
	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Version != 14 {
		t.Errorf("Version: got %d, want 14", meta.Version)
	}
	if meta.Author != "Bob" {
		t.Errorf("Author: got %q, want %q", meta.Author, "Bob")
	}
}

func TestExtractMetadata_AccentedAuthor(t *testing.T) {
	// Author name with CP1252 hex-encoded é (0xe9)
	rtf := `{\rtf1\ansi\ansicpg1252{\info{\author Ren\'e9 M\'fcller}}}`
	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Ren\u00e9 M\u00fcller"
	if meta.Author != want {
		t.Errorf("Author: got %q, want %q", meta.Author, want)
	}
}

func TestExtractMetadata_UnicodeTitle(t *testing.T) {
	// Title with \uN unicode escape
	rtf := `{\rtf1\ansi{\info{\title Caf\u233?}}}body text`
	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Caf\u00e9"
	if meta.Title != want {
		t.Errorf("Title: got %q, want %q", meta.Title, want)
	}
}

func TestExtractMetadata_StopsAfterInfo(t *testing.T) {
	// Build a large document body after the info block to verify
	// the parser stops early and doesn't read the whole stream.
	var sb strings.Builder
	sb.WriteString(`{\rtf1\ansi{\info{\title Early}}`)
	for i := 0; i < 100_000; i++ {
		sb.WriteString(`Some body text here. `)
	}
	sb.WriteString(`}`)

	// Wrap in a counting reader to verify we don't read everything.
	cr := &countingReader{r: strings.NewReader(sb.String())}
	meta, err := ExtractMetadata(cr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Title != "Early" {
		t.Errorf("Title: got %q", meta.Title)
	}
	// The info block is well under 1KB; we should stop long before the body.
	// But the bufio.Reader will read as much as its buffer size which is 32*1024
	if cr.n > 32*1024 {
		t.Errorf("read %d bytes — should have stopped after \\info group (~<2KB)", cr.n)
	}
}

func TestExtractMetadata_EmptyInfo(t *testing.T) {
	rtf := `{\rtf1\ansi{\info}body}`
	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Title != "" || meta.Author != "" {
		t.Errorf("expected empty metadata, got title=%q author=%q", meta.Title, meta.Author)
	}
}

func TestExtractMetadata_NoInfo(t *testing.T) {
	rtf := `{\rtf1\ansi Hello, World!}`
	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Title != "" || meta.Author != "" || meta.Created != nil {
		t.Errorf("expected all-zero metadata, got %+v", meta)
	}
}

func TestExtractMetadata_SpecialCharsInTitle(t *testing.T) {
	rtf := `{\rtf1\ansi{\info{\title Notes \emdash  Summary}}}`
	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "Notes \u2014 Summary"
	if meta.Title != want {
		t.Errorf("Title: got %q, want %q", meta.Title, want)
	}
}

func TestExtractMetadata_AllFields(t *testing.T) {
	rtf := `{\rtf1\ansi{\info
{\title T}{\subject S}{\author A}{\manager M}{\company C}
{\operator O}{\category Cat}{\keywords kw1 kw2}
{\comment Comm}{\doccomm DocC}{\hlinkbase https://example.com}
\version7
}}`
	meta, err := ExtractMetadata(strings.NewReader(rtf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Title != "T" {
		t.Errorf("Title=%q", meta.Title)
	}
	if meta.Subject != "S" {
		t.Errorf("Subject=%q", meta.Subject)
	}
	if meta.Author != "A" {
		t.Errorf("Author=%q", meta.Author)
	}
	if meta.Manager != "M" {
		t.Errorf("Manager=%q", meta.Manager)
	}
	if meta.Company != "C" {
		t.Errorf("Company=%q", meta.Company)
	}
	if meta.Operator != "O" {
		t.Errorf("Operator=%q", meta.Operator)
	}
	if meta.Category != "Cat" {
		t.Errorf("Category=%q", meta.Category)
	}
	if meta.Keywords != "kw1 kw2" {
		t.Errorf("Keywords=%q", meta.Keywords)
	}
	if meta.Comment != "Comm" {
		t.Errorf("Comment=%q", meta.Comment)
	}
	if meta.DocComm != "DocC" {
		t.Errorf("DocComm=%q", meta.DocComm)
	}
	if meta.HLinkBase != "https://example.com" {
		t.Errorf("HLinkBase=%q", meta.HLinkBase)
	}
	if meta.Version != 7 {
		t.Errorf("Version=%d", meta.Version)
	}
}

// countingReader wraps a reader and counts bytes read.
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}
