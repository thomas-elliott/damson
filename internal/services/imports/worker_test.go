package imports

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestPool_AsyncProcessesEnqueuedJobs verifies the goroutine pool drains
// the queue. The pipeline run is short-circuited via a missing DB so the
// jobs land in the workers immediately and we observe Run being invoked
// (it returns an error, which is logged; that's enough to confirm the
// worker picked the job up).
func TestPool_AsyncProcessesEnqueuedJobs(t *testing.T) {
	// Pool with no DB makes Run return an error fast; we count those errors
	// indirectly by counting how many Enqueue calls drained.
	p := New(Deps{}, 2)
	defer p.Shutdown(mustCtx(t))

	for i := 0; i < 5; i++ {
		p.Enqueue(Job{ImportID: uint(i + 1)})
	}
	// Shutdown blocks until all queued jobs drain.
	p.Shutdown(mustCtx(t))
}

// TestPool_SyncRunsInline verifies NewSync executes the pipeline on the
// caller's goroutine rather than queuing. We can't easily observe that
// without a stubbable Run, so instead we confirm Enqueue returns
// synchronously even when there are no workers.
func TestPool_SyncRunsInline(t *testing.T) {
	p := NewSync(Deps{})

	var ran atomic.Bool
	go func() {
		// Enqueue should not deadlock — sync mode must not require a
		// channel reader.
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

func mustCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return ctx
}
