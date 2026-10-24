package preview

import (
	"sync"
	"testing"
)

// An SSE client disconnecting unsubscribes while a preview job publishes. The
// send must never reach a channel the unsubscribe has closed.
func TestBroadcaster_UnsubscribeDuringPublishDoesNotPanic(t *testing.T) {
	b := NewBroadcaster()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for p := 0; p < 4; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					b.Publish(1, Event{VersionID: 1})
				}
			}
		}()
	}
	for i := 0; i < 20000; i++ {
		_, unsub := b.Subscribe(1)
		unsub()
	}
	close(stop)
	wg.Wait()
}
