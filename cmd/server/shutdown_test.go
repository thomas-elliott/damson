package main

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thomas-elliott/damson/internal/services/imports"
)

// recordingQueue notes whether the in-flight request had finished when the
// queue was stopped.
type recordingQueue struct {
	requestDone    *atomic.Bool
	stoppedInOrder bool
}

func (q *recordingQueue) Stop(context.Context) error {
	q.stoppedInOrder = q.requestDone.Load()
	return nil
}

// An open SSE stream must not hold shutdown to its timeout, and the queue
// stops only after HTTP, so a request still in flight can enqueue its job.
func TestShutdown_EndsSSEStreamsThenStopsTheQueue(t *testing.T) {
	b := imports.NewBroadcaster()
	streaming := make(chan struct{})
	var requestDone atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		defer requestDone.Store(true)
		ch, unsub := b.Subscribe(1)
		defer unsub()
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(streaming)
		for {
			select {
			case <-r.Context().Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
			}
		}
	})
	srv := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	<-streaming

	q := &recordingQueue{requestDone: &requestDone}
	const timeout = 2 * time.Second
	start := time.Now()
	shutdown(srv, q, timeout, b.Close)
	if took := time.Since(start); took >= timeout {
		t.Errorf("shutdown took %v; the open stream held it to the timeout", took)
	}
	if !q.stoppedInOrder {
		t.Error("the queue stopped while a request was still in flight")
	}
}
