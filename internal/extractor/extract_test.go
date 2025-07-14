package extractor

import (
	// "net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/johbar/text-extraction-service/v2/internal/cache"
	"github.com/johbar/text-extraction-service/v2/internal/config"
	"github.com/johbar/text-extraction-service/v2/internal/docfactory"
	"github.com/johbar/text-extraction-service/v2/pkg/tesswrap"
)

const readmeOcrPath = "../../pkg/pdflibwrappers/testdata/readme.pdf"

func TestWriteTextOrRunOcr(t *testing.T) {
	conf, err := config.NewTesConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	tesswrap.Languages = "eng"
	df := docfactory.New(conf, nil)
	extract := New(conf, df, &cache.NopCache{}, nil, nil)
	f, err := os.Open(readmeOcrPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	doc, err := df.NewDocFromStream(f, -1, readmeOcrPath)
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	err = extract.WriteTextOrRunOcr(doc, &sb, readmeOcrPath)
	if err != nil {
		t.Error(err)
	}
	t.Log(sb.String())
	if sb.Len() < 100 {
		t.Error("expected more text as a result of OCR")
	}
	if len(doc.Path()) > 0 {
		// temp file
		os.Remove(doc.Path())
	}
}

// func TestExtractRemote(t *testing.T) {
// 	httptest.NewServer()
// }
