// Package mmappool implements a pool of fixed-size off-heap byte slices, backed by anonymous memory-mapped files.
package mmappool

import (
	"log/slog"
	"sync/atomic"

	"github.com/edsrzf/mmap-go"
)

type Mempool struct {
	mmaps      chan mmap.MMap
	elemSize   int
	NumCreated atomic.Int32
	log        *slog.Logger
}

// New creates a new memory pool of max size `poolsize` which emits []byte of size and capacity `elemsize`.
// The pool is lazily populated whenever a byte slice is requested.
// poolsize*elemsize the upper bound of memory that will not be freed up until [Free] is called.
func New(elemSize, poolSize int, logger *slog.Logger) *Mempool {
	if elemSize < 8 {
		panic("illegal elemSize for mempool")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	ch := make(chan mmap.MMap, poolSize)
	return &Mempool{mmaps: ch, elemSize: elemSize, log: logger}
}

// Get returns a byte slice (mmap) from the pool. The pool will create a new one, if none is available for use,
// no matter if this results in exceeding the pool size. Mmaps created beyond the poolsize will be free (unmapped)
// when they are returned to the pool by calling [Put]. The caller is
// responsible for calling [Put]. Not doing so will result in a memory leak.
// If creating an off-heap chunk of memory fails, an ordinary []byte will be returned instead. Additionally
// the error will be returned, contrary to the Go rule to either return a value or an error.
// Slices returned by Get should not be resliced regarding the lower bound.
// It is also illegal to grow the underlying array
func (m *Mempool) Get() ([]byte, error) {
	select {
	case mmap := <-m.mmaps:
		return mmap[:m.elemSize], nil
	default:
		b, err := mmap.MapRegion(nil, m.elemSize, mmap.RDWR, mmap.ANON, 0)
		created := m.NumCreated.Add(1)
		if err != nil {
			return make([]byte, m.elemSize), err
		}
		if created > int32(m.PoolSize()) {
			m.log.Warn("Number of byte slices allocated is bigger than pool size. This might indicate a memory leak.", "created", created, "poolSize", m.PoolSize())
		}
		return b, err
	}
}

// Put returns a byte slice (mmap) to the pool. If all slots are taken the mmap will be freed.
func (m *Mempool) Put(b []byte) {
	if cap(b) != m.elemSize {
		m.log.Debug("discarding buffer with wrong cap", "cap", cap(b))
		// this does not belong here
		return
	}
	select {
	case m.mmaps <- b[:m.elemSize]:
		m.log.Debug("buffer returned to pool", "len", len(b), "cap", cap(b))
		return
	default:
		mmapB := mmap.MMap(b[:m.elemSize])
		err := mmapB.Unmap()
		m.log.Debug("buffer was unmapped because pool was full", "len", len(b), "cap", cap(b), "err", err)
	}
}

// CurrentSize reports the number of allocated byte slices ready to use.
func (m *Mempool) CurrentSize() int {
	return len(m.mmaps)
}

// PoolSize returns the size of the pool
func (m *Mempool) PoolSize() int {
	return cap(m.mmaps)
}

// ElemSize returns the size of each element in the pool.
func (m *Mempool) ElemSize() int {
	return m.elemSize
}

// Free releases every mmap or []byte in the pool.
// Calling [Get] after calling [Free] will allocate new mmap.
// Free reports errors when the pool's elements were not actually mmaps.
func (m *Mempool) Free() []error {
	errs := make([]error, 0, m.CurrentSize())
	for {
		select {
		case b := <-m.mmaps:
			mmapB := mmap.MMap(b[:m.elemSize])
			err := mmapB.Unmap()
			if err != nil {
				errs = append(errs, err)
			}
		default:
			return errs
		}
	}
}
