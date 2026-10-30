package bot

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/happyx-app/HappyX.Bot/internal/api"
)

// fakeDispatches is the API's dispatch claims: a period is claimed once until it is released.
type fakeDispatches struct {
	mu                         sync.Mutex
	held                       map[string]bool
	claims, finishes, releases int
}

func (f *fakeDispatches) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	const key = "survey/2026-10-30"
	switch r.Method {
	case http.MethodPost:
		f.claims++
		if f.held[key] {
			w.WriteHeader(http.StatusConflict)
			io.WriteString(w, `{"error":"already dispatched"}`)
			return
		}
		f.held[key] = true
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{}`)
	case http.MethodPut:
		f.finishes++
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		f.releases++
		delete(f.held, key)
		w.WriteHeader(http.StatusNoContent)
	}
}

func TestDispatchSkipsAndRetries(t *testing.T) {
	f := &fakeDispatches{held: map[string]bool{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	now := time.Date(2026, 10, 30, 13, 0, 0, 0, time.UTC)
	b := &Bot{api: api.New(srv.URL, "key"), log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: func() time.Time { return now }, skip: map[string]time.Time{}}
	g := api.Guild{DiscordID: "100000000000000001"}
	ctx := context.Background()

	sends := 0
	result := delivery{recipients: 2, failed: 2}
	resultErr := errors.New("could not DM any of 2 members")
	run := func() {
		b.dispatch(ctx, g, "survey", "2026-10-30", func() (delivery, error) { sends++; return result, resultErr })
	}

	run()
	if sends != 1 || f.releases != 1 || f.finishes != 1 {
		t.Fatalf("a send that reached nobody gives its claim back: sends %d releases %d finishes %d", sends, f.releases, f.finishes)
	}
	now = now.Add(time.Minute)
	run()
	if f.claims != 1 {
		t.Fatalf("waits before retrying, got %d claims", f.claims)
	}

	now = now.Add(retryAfter)
	result, resultErr = delivery{recipients: 2, failed: 1}, nil
	run()
	if sends != 2 || f.releases != 1 {
		t.Fatalf("retry: sends %d releases %d", sends, f.releases)
	}
	for i := 0; i < 60; i++ {
		now = now.Add(time.Minute)
		run()
	}
	if f.claims != 2 || sends != 2 {
		t.Fatalf("a sent period isn't claimed again: claims %d sends %d", f.claims, sends)
	}

	// Another instance sent it: one 409, then no more claims.
	b.skip = map[string]time.Time{}
	run()
	run()
	if f.claims != 3 || sends != 2 {
		t.Fatalf("claimed elsewhere: claims %d sends %d", f.claims, sends)
	}
}

func TestDispatchKeepsClaimAfterPartialSend(t *testing.T) {
	f := &fakeDispatches{held: map[string]bool{}}
	srv := httptest.NewServer(f)
	defer srv.Close()
	b := &Bot{api: api.New(srv.URL, "key"), log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		now: time.Now, skip: map[string]time.Time{}}
	b.dispatch(context.Background(), api.Guild{DiscordID: "100000000000000001"}, "survey", "2026-10-30",
		func() (delivery, error) { return delivery{recipients: 3, failed: 1}, errors.New("half way") })
	if f.releases != 0 {
		t.Fatal("a send that reached somebody must not be retried")
	}
}
