package main

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/thomas-elliott/damson/internal/auth"
	"github.com/thomas-elliott/damson/internal/config"
	"github.com/thomas-elliott/damson/internal/handlers/api"
	"github.com/thomas-elliott/damson/internal/ingest"
	"github.com/thomas-elliott/damson/internal/models"
	"github.com/thomas-elliott/damson/internal/preview"
	"github.com/thomas-elliott/damson/internal/testdb"
)

// recordingQueue notes whether the in-flight requests had finished when the
// queue was stopped.
type recordingQueue struct {
	streamsDone    *atomic.Int32
	stoppedInOrder bool
}

func (q *recordingQueue) Stop(context.Context) error {
	q.stoppedInOrder = q.streamsDone.Load() == 2
	return nil
}

// userAuthenticator signs every bearer in as one user.
type userAuthenticator struct{ user *models.User }

func (a userAuthenticator) Authenticate(context.Context, string) (*models.User, error) {
	return a.user, nil
}

// An open SSE stream must not hold shutdown to its timeout, and the queue
// stops only after HTTP, so a request still in flight can enqueue its job.
// Both event streams are the real handlers, on a server built as run builds it.
func TestShutdown_EndsSSEStreamsThenStopsTheQueue(t *testing.T) {
	db := testdb.NewPostgres(t)
	lib := models.Library{Name: "Main"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatalf("library: %v", err)
	}
	user := models.User{Email: "sse@example.com", DisplayName: "SSE", CurrentLibraryID: &lib.ID}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := db.Create(&models.LibraryMembership{LibraryID: lib.ID, UserID: user.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("membership: %v", err)
	}
	h := api.New(api.Deps{
		DB: db, AuthMode: config.AuthModeOIDC,
		Broadcaster:        ingest.NewBroadcaster(),
		PreviewBroadcaster: preview.NewBroadcaster(),
	})
	r, err := newRouter("", nil)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	var streamsDone atomic.Int32
	r.Use(func(c *gin.Context) {
		c.Next()
		streamsDone.Add(1)
	})
	api.AddRoutes(r.Group("/api"), auth.Middleware(userAuthenticator{&user}, nil), h)

	srv, stopStreams := newHTTPServer("", r)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()

	for _, path := range []string{"/api/imports/events", "/api/previews/events"} {
		req, _ := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+path, nil)
		req.Header.Set("Authorization", "Bearer any")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", path, resp.StatusCode)
		}
		// The stream is open once the handler has flushed its snapshot; a
		// read now would block until the stream ends.
		go func() { _, _ = bufio.NewReader(resp.Body).ReadString(0) }()
	}

	q := &recordingQueue{streamsDone: &streamsDone}
	const timeout = 2 * time.Second
	start := time.Now()
	shutdown(srv, q, timeout, stopStreams)
	if took := time.Since(start); took >= timeout {
		t.Errorf("shutdown took %v; an open stream held it to the timeout", took)
	}
	if !q.stoppedInOrder {
		t.Errorf("the queue stopped when %d of 2 streams had finished", streamsDone.Load())
	}
}

// The stopping signal ends streams only: a request context built from the
// server's base context is not cancelled by it, so uploads still finish.
func TestNewHTTPServer_StoppingLeavesRequestContextsLive(t *testing.T) {
	srv, stopStreams := newHTTPServer("", http.NotFoundHandler())
	ctx := srv.BaseContext(nil)
	stopStreams()
	if ctx.Err() != nil {
		t.Fatal("stopping the streams cancelled every request")
	}
}
