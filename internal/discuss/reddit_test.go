package discuss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRedditFetcherTokenManagement(t *testing.T) {
	var callCount atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/access_token" {
			callCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token": "abc", "expires_in": 3600}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	f := NewRedditFetcher("id", "secret")
	f.baseURL = s.URL
	f.client = s.Client()

	// Concurrent calls to fetchToken should only hit the server once.
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			tok, err := f.fetchToken(context.Background())
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tok != "abc" {
				t.Errorf("expected token abc, got %s", tok)
			}
		})
	}
	wg.Wait()

	if got := callCount.Load(); got != 1 {
		t.Errorf("expected 1 call to server, got %d", got)
	}

	// Should still be 1 call after more sequential calls.
	for range 5 {
		_, _ = f.fetchToken(context.Background())
	}
	if got := callCount.Load(); got != 1 {
		t.Errorf("expected 1 call to server after sequential calls, got %d", got)
	}

	// Invalidate it.
	f.token.Store(nil)
	_, _ = f.fetchToken(context.Background())
	if got := callCount.Load(); got != 2 {
		t.Errorf("expected 2 calls after invalidation, got %d", got)
	}
}
