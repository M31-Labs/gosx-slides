package slides

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestPresenterBrokerRelays: a published position reaches a subscriber, current()
// reflects the latest, and publishing after unsubscribe is safe (closed channel
// not touched).
func TestPresenterBrokerRelays(t *testing.T) {
	b := newPresenterBroker()
	ch := b.subscribe()

	b.publish(presenterState{Index: 3, Step: 1})
	select {
	case s := <-ch:
		if s.Index != 3 || s.Step != 1 {
			t.Fatalf("relayed %+v, want {3 1}", s)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber received no state")
	}

	if cur := b.current(); cur.Index != 3 || cur.Step != 1 {
		t.Fatalf("current() = %+v, want {3 1}", cur)
	}

	b.unsubscribe(ch)
	b.publish(presenterState{Index: 4}) // must not panic on the unsubscribed channel
	if cur := b.current(); cur.Index != 4 {
		t.Fatalf("current() = %+v after publish, want index 4", cur)
	}
}

// TestPresenterHandleStatePublishes: POST /presenter/state parses JSON and fans
// the position out to subscribers (the path the phone remote and presenter use).
func TestPresenterHandleStatePublishes(t *testing.T) {
	b := newPresenterBroker()
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	req := httptest.NewRequest("POST", "/presenter/state", strings.NewReader(`{"index":7,"step":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	b.handleState(rec, req)
	if rec.Code != 204 {
		t.Fatalf("handleState status = %d, want 204", rec.Code)
	}
	select {
	case s := <-ch:
		if s.Index != 7 || s.Step != 2 {
			t.Fatalf("published %+v, want {7 2}", s)
		}
	case <-time.After(time.Second):
		t.Fatal("handleState did not publish to subscribers")
	}

	// A GET must be rejected (only POST publishes).
	rec = httptest.NewRecorder()
	b.handleState(rec, httptest.NewRequest("GET", "/presenter/state", nil))
	if rec.Code != 405 {
		t.Fatalf("GET handleState status = %d, want 405", rec.Code)
	}
}
