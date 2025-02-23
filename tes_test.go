package main

import (
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/johbar/text-extraction-service/v2/pkg/pdflibwrappers/poppler_purego"
)

func TestWriteTextOrRunOcr(t *testing.T) {
	// we need to create a logger, otherwise it's a nil pointer
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug, AddSource: true}))
	if _, err := poppler_purego.InitLib(""); err != nil {
		return
	}

	data, err := os.ReadFile("pkg/pdflibwrappers/testdata/readme.pdf")
	if err != nil {
		panic(err)
	}
	doc, err := poppler_purego.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	err = WriteTextOrRunOcr(doc, &sb, "readme.pdf")
	if err != nil {
		t.Error(err)
	}
	t.Log(sb.String())
	if sb.Len() < 100 {
		t.Error("expected more text as a result of OCR")
	}
}

