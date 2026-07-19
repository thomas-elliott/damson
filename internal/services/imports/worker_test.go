package imports

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestPool_SyncRunsInline verifies NewSync executes the pipeline on the
// caller's goroutine rather than queuing. We can't easily observe that
// without a stubbable Run, so instead we confirm Enqueue returns
// synchronously even when there are no workers.
func TestPool_SyncRunsInline(t *testing.T) {
	p := NewSync(Deps{})

	var ran atomic.Bool
	go func() {
		// Enqueue should not deadlock — sync mode runs inline.
		p.Enqueue(Job{ImportID: 1})
		ran.Store(true)
	}()

	deadline := time.After(time.Second)
	for !ran.Load() {
		select {
		case <-deadline:
			t.Fatalf("sync Enqueue did not return")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
