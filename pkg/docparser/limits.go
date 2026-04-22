package docparser

// limits.go — resource limits applied when parsing untrusted files.
//
// All limits are intentionally conservative: they should never be hit by any
// legitimate document, but they bound memory allocation and CPU time when a
// crafted file is supplied.

import "fmt"

const (
	// maxPieceCount is the maximum number of pieces (Pcd entries) accepted
	// in a DOC piece table. Real documents rarely exceed a few thousand even
	// for heavily-edited files; 1 million is a generous ceiling that still
	// prevents a crafted lcb from causing a multi-gigabyte allocation.
	maxPieceCount = 1_000_000

	// maxPieceBytes is the maximum byte length of a single piece buffer
	// (compressed or uncompressed). A legitimate paragraph or text run never
	// approaches 10 MB; this cap prevents a single large recLen from
	// exhausting memory.
	maxPieceBytes = 10 * 1024 * 1024 // 10 MB

	// maxPersistDirEntries is the maximum total number of entries accepted
	// across all PersistDirectoryAtom records. A presentation with thousands
	// of slides would still be well under 100 000; this cap prevents a
	// crafted cPersist from filling the map unboundedly.
	maxPersistDirEntries = 100_000

	// maxUserEditChain is the maximum number of UserEditAtom records followed
	// when building the persist object directory. Each incremental save appends
	// one record; even a document that has been saved ten thousand times is
	// pathological. This cap prevents crafted offsetLastEdit chains from
	// looping indefinitely.
	maxUserEditChain = 10_000

	// maxSlides is the maximum number of presentation slides extracted.
	// No legitimate presentation approaches this count.
	maxSlides = 10_000
)

// errLimit returns a formatted error for a limit violation.
func errLimit(what string, limit int) error {
	return fmt.Errorf("limit exceeded: %s (max %d)", what, limit)
}
