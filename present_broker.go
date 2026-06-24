package slides

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// present_broker.go is the real-lane cross-device presenter: a tiny in-process
// broker plus three endpoints mounted on the deck's server.App. It replaces the
// browser-local BroadcastChannel-only sync with real cross-machine lockstep — a
// presenter window (or phone /remote) publishes {slide, step}, every connected
// audience follows over Server-Sent Events. One deck = one room.
//
// Why SSE over app.Mount (not Page/Route): a Mounted handler receives the raw,
// flushable http.ResponseWriter; Page/Route buffer the whole response into a
// non-flushing writer and could never stream. (Verified against gosx server.)

// presenterState is the authoritative position the broker relays. The client
// computes the next position (step-then-slide) and clamps against its own DOM, so
// the server stays a dumb relay.
type presenterState struct {
	Index int `json:"index"`
	Step  int `json:"step"`
}

// presenterBroker fans one published position out to every subscribed SSE client
// and remembers the latest so a fresh/reconnecting client snaps to the live spot.
type presenterBroker struct {
	mu    sync.Mutex
	subs  map[chan presenterState]struct{}
	state presenterState
}

func newPresenterBroker() *presenterBroker {
	return &presenterBroker{subs: map[chan presenterState]struct{}{}}
}

func (b *presenterBroker) subscribe() chan presenterState {
	ch := make(chan presenterState, 8)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *presenterBroker) unsubscribe(ch chan presenterState) {
	b.mu.Lock()
	if _, ok := b.subs[ch]; ok {
		delete(b.subs, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *presenterBroker) current() presenterState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// publish records the new position and non-blockingly fans it out. A slow client
// whose buffer is full simply drops this update (it will re-sync on the next one
// or on reconnect) rather than stalling the presenter.
func (b *presenterBroker) publish(s presenterState) {
	b.mu.Lock()
	b.state = s
	for ch := range b.subs {
		select {
		case ch <- s:
		default:
		}
	}
	b.mu.Unlock()
}

// handleEvents is the SSE stream: it sends the current position immediately (so a
// late joiner snaps to the live slide), then every published change, with a
// heartbeat to keep intermediaries from idling the connection.
func (b *presenterBroker) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Defeat the server's 45s WriteTimeout so the stream isn't torn down mid-talk;
	// EventSource auto-reconnect + replay-on-connect is the backstop if a proxy
	// still cuts it.
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	writeSSEState(w, b.current())
	flusher.Flush()

	ch := b.subscribe()
	defer b.unsubscribe(ch)
	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case s, ok := <-ch:
			if !ok {
				return
			}
			writeSSEState(w, s)
			flusher.Flush()
		case <-heartbeat.C:
			io.WriteString(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeSSEState(w io.Writer, s presenterState) {
	data, _ := json.Marshal(s)
	fmt.Fprintf(w, "event: state\ndata: %s\n\n", data)
}

// handleState publishes a position from the presenter window or the phone remote.
// Accepts JSON or form-encoded {index, step}.
func (b *presenterBroker) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var s presenterState
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&s)
	} else {
		_ = r.ParseForm()
		s.Index, _ = strconv.Atoi(r.FormValue("index"))
		s.Step, _ = strconv.Atoi(r.FormValue("step"))
	}
	b.publish(s)
	w.WriteHeader(http.StatusNoContent)
}

// handleRemote serves the standalone phone-remote page: prev/next/goto that POST
// to /presenter/state, and an EventSource that mirrors the live slide number. It
// is a control surface, not a themed deck, so it carries no island runtime.
func handleRemote(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, remoteHTML)
}

const remoteHTML = `<!doctype html><html><head><meta charset=utf-8>
<meta name=viewport content="width=device-width,initial-scale=1,maximum-scale=1">
<title>slides remote</title>
<style>
  :root{color-scheme:dark}
  body{margin:0;height:100vh;display:flex;flex-direction:column;gap:1rem;align-items:center;justify-content:center;
    background:#0c0f1a;color:#eef1f8;font:600 18px/1.4 system-ui,sans-serif;-webkit-user-select:none;user-select:none}
  .cur{font-size:3rem;font-weight:800;color:#f6b352}
  .row{display:flex;gap:1rem}
  button{font:700 1.1rem system-ui;color:#0c0f1a;background:#f6b352;border:0;border-radius:14px;padding:1.4rem 2.2rem;cursor:pointer}
  button:active{transform:scale(.97)}
  .ghost{background:transparent;color:#9aa3bd;border:1px solid rgba(255,255,255,.15)}
  form{display:flex;gap:.5rem}
  input{width:5rem;font:600 1.1rem system-ui;text-align:center;border-radius:10px;border:1px solid rgba(255,255,255,.15);background:#161b2e;color:#eef1f8;padding:.6rem}
</style></head><body>
  <div>slide <span class=cur id=cur>1</span></div>
  <div class=row>
    <button class=ghost onclick="go(cur-1)">‹ prev</button>
    <button onclick="go(cur+1)">next ›</button>
  </div>
  <form onsubmit="go(parseInt(this.n.value,10)-1);return false">
    <input id=n name=n type=number min=1 placeholder=#>
    <button class=ghost type=submit>go</button>
  </form>
<script>
  var cur = 0;
  function go(i){ if(i<0)i=0; fetch('presenter/state',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({index:i,step:0}),keepalive:true}); }
  try {
    var es = new EventSource('presenter/events');
    es.addEventListener('state', function(e){
      try { var d = JSON.parse(e.data); if (typeof d.index === 'number'){ cur = d.index; document.getElementById('cur').textContent = (cur+1); } } catch(_){}
    });
  } catch(_){}
</script></body></html>`
