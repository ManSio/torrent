package torrent

import (
	"testing"
)

// Storage eviction regression (YouROK/TorrServer#925): when an external cache
// drops a partial piece, the library still counts its chunks as received
// (dirtyChunks) and would serve zeros to responsive readers. After
// InvalidateStorage every chunk must be pending again.
func TestInvalidateStorageClearsDirtyChunks(t *testing.T) {
	cn := chokeFixture(2)
	defer stopChokeFixture(cn)
	cn.t.cl.event.L = cn.t.cl.locker()
	tor := cn.t
	p0 := &tor.pieces[0]
	p1 := &tor.pieces[1]
	numChunks := int(p0.numChunks())

	// Received chunks of piece 0, plus one on the neighbouring piece as a
	// negative control (must remain untouched).
	p0.unpendChunkIndex(0)
	p0.unpendChunkIndex(1)
	p1.unpendChunkIndex(0)

	// Control: while the dirty state exists, the chunk counts as present.
	r0 := request{Index: 0, chunkSpec: tor.chunkIndexSpec(0, 0)}
	if !tor.haveChunk(r0) {
		t.Fatal("control: dirty chunk must count as have before invalidation")
	}
	if p0.dirtyChunks.Len() != 2 {
		t.Fatalf("control: dirtyChunks=%d, want 2", p0.dirtyChunks.Len())
	}
	undirtied := countUndirtiedChunks(tor, 0)
	if undirtied != numChunks-2 {
		t.Fatalf("control: undirtied=%d, want %d", undirtied, numChunks-2)
	}

	p0.InvalidateStorage()

	if p0.dirtyChunks.Len() != 0 {
		t.Fatalf("dirtyChunks=%d after InvalidateStorage, want 0", p0.dirtyChunks.Len())
	}
	if tor.haveChunk(r0) {
		t.Fatal("haveChunk still true for invalidated chunk")
	}
	if undirtied = countUndirtiedChunks(tor, 0); undirtied != numChunks {
		t.Fatalf("undirtied=%d after InvalidateStorage, want all %d", undirtied, numChunks)
	}
	// Negative control: neighbour piece's dirty state is unaffected.
	if p1.dirtyChunks.Len() != 1 {
		t.Fatalf("neighbour piece dirty state changed: %d, want 1", p1.dirtyChunks.Len())
	}
}

// Negative control for the regression itself: without InvalidateStorage the
// dirty (evicted) chunk keeps counting as received. If this assertion fails,
// the fixture no longer models the bug.
func TestWithoutInvalidateStorageDirtyChunkStillCounts(t *testing.T) {
	cn := chokeFixture(2)
	defer stopChokeFixture(cn)
	cn.t.cl.event.L = cn.t.cl.locker()
	tor := cn.t
	tor.pieces[0].unpendChunkIndex(0)
	r0 := request{Index: 0, chunkSpec: tor.chunkIndexSpec(0, 0)}
	if !tor.haveChunk(r0) {
		t.Fatal("fixture no longer models the bug: dirty chunk not counted as have")
	}
}

func countUndirtiedChunks(tor *Torrent, piece pieceIndex) (n int) {
	iterUndirtiedChunks(piece, tor, func(chunkSpec) bool {
		n++
		return true
	})
	return
}
