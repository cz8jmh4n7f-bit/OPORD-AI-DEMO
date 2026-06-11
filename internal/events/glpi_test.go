package events

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestGLPISinkCreatesCIOnReady(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/initSession"):
			_, _ = w.Write([]byte(`{"session_token":"sess"}`))
		case strings.HasSuffix(r.URL.Path, "/killSession"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer srv.Close()

	sink := NewGLPISink(srv.URL, "app-token", "user-token", "Computer")

	// A "ready" event should create a CMDB CI (initSession to POST Computer to killSession).
	if err := sink.Emit(context.Background(), Event{Kind: "vm", Action: "ready", Name: "demo", Provider: "aws-eu"}); err != nil {
		t.Fatalf("emit ready: %v", err)
	}
	mu.Lock()
	got := strings.Join(calls, " | ")
	mu.Unlock()
	if !strings.Contains(got, "/apirest.php/initSession") {
		t.Fatalf("expected initSession call, got: %s", got)
	}
	if !strings.Contains(got, "POST /apirest.php/Computer") {
		t.Fatalf("expected Computer create, got: %s", got)
	}

	// A non-ready event should make no GLPI calls.
	mu.Lock()
	calls = nil
	mu.Unlock()
	if err := sink.Emit(context.Background(), Event{Kind: "vm", Action: "created", Name: "demo"}); err != nil {
		t.Fatalf("emit created: %v", err)
	}
	mu.Lock()
	n := len(calls)
	mu.Unlock()
	if n != 0 {
		t.Fatalf("expected no GLPI calls for non-ready action, got %d", n)
	}
}
