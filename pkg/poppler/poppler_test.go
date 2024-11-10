package puregopoppler

import (
	"io"
	"os"
	"testing"
)
// these tests only assure the purego wrapper works in any way

func TestGetVersion(t *testing.T) {
	got := Version()
	t.Logf("Poppler version is %s", got)
	if len(got) < 1 {
		t.Fail()
	}
}

func TestNewFromBytes(t *testing.T) {
	data := readFile()
	d, err := Load(data)
	if (err != nil) {
		t.Fatal(err)
	}
	t.Logf("document created: %v", d)
	d.Close()
}

func TestGetText(t *testing.T) {
	d, err := Load(readFile())
	if (err != nil) {
		t.Fatal(err)
	}
	p := d.GetPage(0)
	txt := p.Text()
	p.Close()
	t.Log(txt)
}
func TestInfo(t *testing.T) {
	d, err := Load(readFile())
	if (err != nil) {
		t.Fatal(err)
	}
	i := d.Info()
	t.Log(i)
}
func readFile() []byte {
	f, _ := os.Open("testdata/2000001.pdf")
	data, _ := io.ReadAll(f)
	f.Close()
	return data
}
