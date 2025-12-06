package mmappool_test

import (
	"log/slog"
	"os"
	"testing"

	"github.com/edsrzf/mmap-go"
	"github.com/johbar/text-extraction-service/v4/pkg/mmappool"
)

func TestMemPool(t *testing.T) {
	poolsize := 10
	mp := mmappool.New(256, poolsize, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	bufs := make([][]byte, 0, poolsize)
	for range poolsize {
		b, err := mp.Get()
		if err != nil {
			t.Errorf("getting element from mempool: %v", err)
		}
		bufs = append(bufs, b)
		if cap(b) != mp.ElemSize() {
			t.Errorf("got: %d, want %d elemsize", cap(b), mp.ElemSize())
		}
		if err := mmap.MMap(b).Flush(); err != nil {
			t.Error("buffer is not a mmap!")
		}
	}
	for i, b := range bufs {
		mp.Put(b)
		if mp.CurrentSize() != i+1 {
			t.Errorf("got: %v, want: %v", mp.CurrentSize(), i+1)
		}
	}
}

func TestReslicingBuffers(t *testing.T) {
	mp := mmappool.New(256, 1, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	b, err := mp.Get()
	if err != nil {
		t.Error(err)
	}
	b = b[:128]
	mp.Put(b)
	errs := mp.Free()
	if len(errs) > 0 {
		t.Errorf("reslicing is forbidden, %v", errs)
	}

}

func TestMemPoolFree(t *testing.T) {
	poolsize := 10
	mp := mmappool.New(256, poolsize, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	bufs := make([][]byte, 0, poolsize)
	for range poolsize {
		b, err := mp.Get()
		if err != nil {
			t.Error(err)
		}
		bufs = append(bufs, b)
	}
	for _, buf := range bufs {
		mp.Put(buf)
	}
	if mp.CurrentSize() != poolsize {
		t.Errorf("got: %v, want: %v", mp.CurrentSize(), poolsize)
	}
	errs := mp.Free()
	if len(errs) != 0 {
		t.Errorf("got: %v", errs)
	}
	if mp.CurrentSize() > 0 {
		t.Errorf("got: %v, want: 0", mp.CurrentSize())
	}
}

func BenchmarkNormalAlloc(b *testing.B) {
	for b.Loop() {
		buf := make([]byte, 200_000)
		buf[0] = 1
	}
}

func BenchmarkMmapAlloc(b *testing.B) {
	for b.Loop() {
		m, err := mmap.MapRegion(nil, 200_000, mmap.RDWR, mmap.ANON, 0)
		if err != nil {
			b.Error(err)
		}
		m[0] = 1
	}
}

func BenchmarkMempoolAlloc(b *testing.B) {
	pool := mmappool.New(200_000, 20_000, nil)
	for b.Loop() {
		buf, err := pool.Get()
		if err != nil {
			b.Fatal(err)
		}
		buf[0] = 1
		pool.Put(buf)
	}
}
