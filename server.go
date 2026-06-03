package slides

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ServerOptions configures dev and presenter serving.
type ServerOptions struct {
	Mode string
	Addr string
}

type presentationState struct {
	SlideIndex      int  `json:"slideIndex"`
	ClickStep       int  `json:"clickStep"`
	AudienceCount   int  `json:"audienceCount"`
	UpdatedAtUnix   int  `json:"updatedAtUnix"`
	StartedAtUnix   int  `json:"startedAtUnix"`
	Paused          bool `json:"paused"`
	PausedAtUnix    int  `json:"pausedAtUnix"`
	PausedTotalSecs int  `json:"pausedTotalSecs"`
}

type event struct {
	Name string
	Data string
}

type broker struct {
	mu      sync.Mutex
	clients map[chan event]struct{}
}

func newBroker() *broker {
	return &broker{clients: map[chan event]struct{}{}}
}

func (b *broker) subscribe() chan event {
	ch := make(chan event, 12)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan event) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}

func (b *broker) publish(ev event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (b *broker) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients)
}

// Serve starts the deck runtime and blocks until the HTTP server exits.
func Serve(deckPath string, opts ServerOptions) error {
	if opts.Mode == "" {
		opts.Mode = "dev"
	}
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:8080"
	}
	if _, err := os.Stat(deckPath); err != nil {
		return err
	}

	events := newBroker()
	now := int(time.Now().Unix())
	state := &presentationState{UpdatedAtUnix: now, StartedAtUnix: now}
	var stateMu sync.Mutex

	loadDeck := func() (*Deck, error) {
		return ParseFile(deckPath)
	}
	statePayload := func() string {
		stateMu.Lock()
		defer stateMu.Unlock()
		state.AudienceCount = events.count()
		state.UpdatedAtUnix = int(time.Now().Unix())
		payload, _ := json.Marshal(state)
		return string(payload)
	}
	publishState := func() {
		events.publish(event{Name: "state", Data: statePayload()})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		ch := events.subscribe()
		defer events.unsubscribe(ch)
		fmt.Fprintf(w, "event: state\ndata: %s\n\n", statePayload())
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case ev := <-ch:
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Name, ev.Data)
				flusher.Flush()
			}
		}
	})
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, statePayload())
	})
	mux.HandleFunc("/api/analysis", func(w http.ResponseWriter, r *http.Request) {
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Analyze(deck))
	})
	mux.HandleFunc("/api/command", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var cmd struct {
			Action     string `json:"action"`
			SlideIndex int    `json:"slideIndex"`
		}
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			_ = json.NewDecoder(r.Body).Decode(&cmd)
		} else {
			_ = r.ParseForm()
			cmd.Action = r.Form.Get("action")
			cmd.SlideIndex, _ = strconv.Atoi(r.Form.Get("slideIndex"))
		}
		stateMu.Lock()
		applyCommand(deck, state, cmd.Action, cmd.SlideIndex)
		stateMu.Unlock()
		publishState()
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/presenter", func(w http.ResponseWriter, r *http.Request) {
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, RenderPresenterHTML(deck, statePayload()))
	})
	mux.HandleFunc("/remote", func(w http.ResponseWriter, r *http.Request) {
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, RenderRemoteHTML(deck, statePayload()))
	})
	mux.HandleFunc("/deck.json", func(w http.ResponseWriter, r *http.Request) {
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(exportManifest(deck))
	})
	mux.HandleFunc("/notes", func(w http.ResponseWriter, r *http.Request) {
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, renderNotesHTML(deck))
	})
	mux.HandleFunc("/handout", func(w http.ResponseWriter, r *http.Request) {
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, renderHandoutHTML(deck))
	})
	mux.Handle("/public/", http.StripPrefix("/public/", http.FileServer(http.Dir(filepath.Join(deckBaseDir(deckPath), "public")))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		deck, err := loadDeck()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		mode := "deck"
		live := opts.Mode == "dev"
		if opts.Mode == "present" {
			mode = "audience"
			live = true
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, RenderDeckHTML(deck, RenderOptions{Mode: mode, LiveReload: live}))
	})

	if opts.Mode == "dev" {
		go watchDeck(deckPath, events)
	}

	fmt.Printf("slides %s serving %s at http://%s\n", opts.Mode, deckPath, opts.Addr)
	if opts.Mode == "present" {
		fmt.Printf("presenter: http://%s/presenter\nremote: http://%s/remote\n", opts.Addr, opts.Addr)
	}
	err := http.ListenAndServe(opts.Addr, mux)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func applyCommand(deck *Deck, state *presentationState, action string, target int) {
	if len(deck.Slides) == 0 {
		return
	}
	now := int(time.Now().Unix())
	maxSlide := len(deck.Slides) - 1
	if state.SlideIndex > maxSlide {
		state.SlideIndex = maxSlide
	}
	maxStep := deck.Slides[state.SlideIndex].Clicks
	switch action {
	case "start":
		state.StartedAtUnix = now
		state.PausedAtUnix = 0
		state.PausedTotalSecs = 0
		state.Paused = false
	case "pause":
		if !state.Paused {
			state.Paused = true
			state.PausedAtUnix = now
		}
	case "resume":
		if state.Paused {
			state.PausedTotalSecs += now - state.PausedAtUnix
			state.PausedAtUnix = 0
			state.Paused = false
		}
	case "reset":
		state.SlideIndex = 0
		state.ClickStep = 0
		state.StartedAtUnix = now
		state.PausedAtUnix = 0
		state.PausedTotalSecs = 0
		state.Paused = false
	case "next":
		if state.ClickStep < maxStep {
			state.ClickStep++
		} else if state.SlideIndex < maxSlide {
			state.SlideIndex++
			state.ClickStep = 0
		}
	case "prev":
		if state.ClickStep > 0 {
			state.ClickStep--
		} else if state.SlideIndex > 0 {
			state.SlideIndex--
			state.ClickStep = deck.Slides[state.SlideIndex].Clicks
		}
	case "goto":
		if target < 0 {
			target = 0
		}
		if target > maxSlide {
			target = maxSlide
		}
		state.SlideIndex = target
		state.ClickStep = 0
	}
}

func watchDeck(deckPath string, events *broker) {
	last, err := deckModTime(deckPath)
	if err != nil {
		return
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		next, err := deckModTime(deckPath)
		if err != nil {
			continue
		}
		if next.After(last) {
			last = next
			events.publish(event{Name: "reload", Data: "{}"})
		}
	}
}

func deckBaseDir(path string) string {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return path
	}
	return filepath.Dir(path)
}

func deckModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	if !info.IsDir() {
		return info.ModTime(), nil
	}
	latest := info.ModTime()
	for _, rel := range []string{"deck.md", "index.md"} {
		if fileInfo, err := os.Stat(filepath.Join(path, rel)); err == nil && fileInfo.ModTime().After(latest) {
			latest = fileInfo.ModTime()
		}
	}
	files, _ := filepath.Glob(filepath.Join(path, "slides", "*.md"))
	for _, file := range files {
		if fileInfo, err := os.Stat(file); err == nil && fileInfo.ModTime().After(latest) {
			latest = fileInfo.ModTime()
		}
	}
	return latest, nil
}

// RenderPresenterHTML renders the presenter console.
func RenderPresenterHTML(deck *Deck, initialState string) string {
	deckJSON, _ := json.Marshal(deckState(deck))
	return "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>" +
		html.EscapeString(deck.Title) + " presenter</title><style>" + baseCSS() + "</style></head><body class=\"presenter theme-" + html.EscapeString(themeClass(deck.Theme)) + "\">" +
		"<div class=\"presenter-grid\"><section class=\"presenter-panel\"><h1>" + html.EscapeString(deck.Title) + "</h1><div class=\"timer\" id=\"presenter-timer\">00:00</div><iframe class=\"presenter-preview\" id=\"presenter-preview\" src=\"/#1\" title=\"Current slide preview\"></iframe><h2 id=\"presenter-current\"></h2><p id=\"presenter-next\"></p><p id=\"presenter-meta\"></p><p id=\"presenter-pace\"></p><div class=\"presenter-controls\">" +
		"<button data-cmd=\"prev\">Prev</button><button data-cmd=\"next\">Next</button><button data-cmd=\"goto\">Goto</button><input id=\"goto-input\" type=\"number\" min=\"1\" value=\"1\" aria-label=\"Slide number\"></div><div class=\"session-controls\"><button data-cmd=\"start\">Start</button><button data-cmd=\"pause\">Pause</button><button data-cmd=\"resume\">Resume</button><button data-cmd=\"reset\">Reset</button></div><div class=\"recording-controls\"><button data-record=\"toggle\">Record</button><button data-record=\"download\">Download Rehearsal</button></div></section>" +
		"<aside class=\"presenter-panel\"><h2>Notes</h2><div class=\"notes-panel\" id=\"presenter-notes\"></div><h2>Run of show</h2><div class=\"slide-list\" id=\"presenter-slide-list\">" + renderPresenterSlideList(deck) + "</div><h2>Checkpoints</h2><div class=\"checkpoint-list\">" + renderPresenterCheckpoints(deck) + "</div><p><a class=\"join-link\" id=\"remote-link\" href=\"/remote\">Remote</a><a class=\"join-link\" id=\"audience-link\" href=\"/\">Audience</a><a class=\"join-link\" href=\"/handout\">Handout</a><a class=\"join-link\" href=\"/notes\">Notes</a><a class=\"join-link\" href=\"/deck.json\">Manifest</a></p></aside></div>" +
		"<script>window.__SLIDES_DECK__=" + string(deckJSON) + ";window.__SLIDES_STATE__=" + initialState + ";</script><script>" + presenterJS() + "</script></body></html>"
}

// RenderRemoteHTML renders the phone remote.
func RenderRemoteHTML(deck *Deck, initialState string) string {
	deckJSON, _ := json.Marshal(deckState(deck))
	return "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"><title>" +
		html.EscapeString(deck.Title) + " remote</title><style>" + baseCSS() + "</style></head><body class=\"remote theme-" + html.EscapeString(themeClass(deck.Theme)) + "\"><main class=\"remote-panel\"><h1>" +
		html.EscapeString(deck.Title) + "</h1><h2 id=\"remote-current\"></h2><p id=\"remote-meta\"></p><div class=\"timer\" id=\"remote-timer\">00:00</div><div class=\"remote-controls\"><button data-cmd=\"prev\">Prev</button><button data-cmd=\"next\">Next</button></div><form id=\"remote-goto\"><input name=\"slide\" type=\"number\" min=\"1\" value=\"1\" aria-label=\"Slide number\"><button>Goto</button></form></main>" +
		"<script>window.__SLIDES_DECK__=" + string(deckJSON) + ";window.__SLIDES_STATE__=" + initialState + ";</script><script>" + remoteJS() + "</script></body></html>"
}

func renderPresenterSlideList(deck *Deck) string {
	var buf strings.Builder
	for _, slide := range deck.Slides {
		buf.WriteString("<button type=\"button\" data-cmd=\"goto\" data-slide-index=\"")
		buf.WriteString(strconv.Itoa(slide.Index))
		buf.WriteString("\"><strong>")
		buf.WriteString(strconv.Itoa(slide.Index + 1))
		buf.WriteString(". ")
		buf.WriteString(html.EscapeString(slide.Title))
		buf.WriteString("</strong><span>")
		buf.WriteString(html.EscapeString(slide.Layout))
		buf.WriteString("</span></button>")
	}
	return buf.String()
}

func renderPresenterCheckpoints(deck *Deck) string {
	var buf strings.Builder
	analysis := Analyze(deck)
	if len(analysis.Checkpoints) == 0 {
		return "<p>No checkpoints.</p>"
	}
	for _, checkpoint := range analysis.Checkpoints {
		slideTitle := ""
		if checkpoint.SlideIndex >= 0 && checkpoint.SlideIndex < len(deck.Slides) {
			slideTitle = deck.Slides[checkpoint.SlideIndex].Title
		}
		buf.WriteString("<button type=\"button\" data-cmd=\"goto\" data-slide-index=\"")
		buf.WriteString(strconv.Itoa(checkpoint.SlideIndex))
		buf.WriteString("\"><strong>")
		buf.WriteString(html.EscapeString(checkpoint.Label))
		buf.WriteString("</strong><span>")
		buf.WriteString(strconv.Itoa(checkpoint.SlideIndex + 1))
		buf.WriteString(". ")
		buf.WriteString(html.EscapeString(slideTitle))
		buf.WriteString("</span></button>")
	}
	return buf.String()
}

func presenterJS() string {
	return `
(function(){
  const deck = window.__SLIDES_DECK__;
  let state = window.__SLIDES_STATE__ || {slideIndex:0, clickStep:0, audienceCount:0};
  const current = document.getElementById('presenter-current');
  const next = document.getElementById('presenter-next');
  const notes = document.getElementById('presenter-notes');
  const meta = document.getElementById('presenter-meta');
  const pace = document.getElementById('presenter-pace');
  const timer = document.getElementById('presenter-timer');
  const preview = document.getElementById('presenter-preview');
  const remoteLink = document.getElementById('remote-link');
  const audienceLink = document.getElementById('audience-link');
  let recording = null;
  let recordingActive = false;
  if (remoteLink) { remoteLink.href = location.origin + '/remote'; remoteLink.textContent = location.origin + '/remote'; }
  if (audienceLink) { audienceLink.href = location.origin + '/'; audienceLink.textContent = location.origin + '/'; }
  function escapeHTML(value) {
    return String(value || '').replace(/[&<>"']/g, ch => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));
  }
  function elapsedSeconds(){
    if (!state.startedAtUnix) return 0;
    const now = Math.floor(Date.now() / 1000);
    const endpoint = state.paused && state.pausedAtUnix ? state.pausedAtUnix : now;
    return Math.max(0, endpoint - state.startedAtUnix - (state.pausedTotalSecs || 0));
  }
  function formatElapsed(total){
    const minutes = Math.floor(total / 60);
    const seconds = total % 60;
    return String(minutes).padStart(2, '0') + ':' + String(seconds).padStart(2, '0');
  }
  function render(){
    const slide = deck.slides[state.slideIndex] || deck.slides[0] || {};
    const nextSlide = deck.slides[state.slideIndex + 1] || {};
    current.textContent = (state.slideIndex + 1) + '. ' + (slide.title || '');
    const previewHash = '#' + (state.slideIndex + 1);
    if (preview && preview.dataset.hash !== previewHash) {
      preview.dataset.hash = previewHash;
      preview.src = '/' + previewHash;
    }
    next.textContent = nextSlide.title ? 'Next: ' + nextSlide.title : 'Final slide';
    notes.innerHTML = escapeHTML(slide.notes || '').replace(/\n/g, '<br>');
    meta.textContent = 'Click ' + (state.clickStep || 0) + ' of ' + (slide.clicks || 0) + ' / audience ' + (state.audienceCount || 0) + (state.paused ? ' / paused' : '');
    if (timer) timer.textContent = formatElapsed(elapsedSeconds());
    if (pace) {
      const maxClicks = slide.clicks || 0;
      const clickFraction = maxClicks > 0 ? (state.clickStep || 0) / (maxClicks + 1) : 0;
      const target = (slide.startSecond || 0) + Math.round((slide.estimatedSeconds || 0) * clickFraction);
      const delta = elapsedSeconds() - target;
      const direction = delta === 0 ? 'on pace' : (delta > 0 ? 'behind' : 'ahead');
      pace.textContent = direction === 'on pace' ? 'On pace' : (Math.abs(delta) + 's ' + direction + ' / target ' + formatElapsed(target));
    }
    document.querySelectorAll('.slide-list [data-slide-index]').forEach(button => {
      button.classList.toggle('is-current', parseInt(button.dataset.slideIndex || '0', 10) === state.slideIndex);
    });
  }
  function recordPoint(action, target){
    if (!recording || !recordingActive) return;
    recording.events.push({
      atSeconds: elapsedSeconds(),
      action,
      targetSlideIndex: target,
      slideIndex: state.slideIndex || 0,
      clickStep: state.clickStep || 0
    });
  }
  function toggleRecording(){
    if (recordingActive) {
      recordPoint('record-stop', state.slideIndex || 0);
      recording.finishedAtUnix = Math.floor(Date.now() / 1000);
      recordingActive = false;
      return false;
    }
    recording = {
      title: deck.title || 'Deck',
      startedAtUnix: Math.floor(Date.now() / 1000),
      estimatedTotalSeconds: deck.estimatedTotalSeconds || 0,
      events: []
    };
    recordingActive = true;
    recordPoint('record-start', state.slideIndex || 0);
    return true;
  }
  function downloadRecording(){
    if (!recording) toggleRecording();
    recording.finishedAtUnix = Math.floor(Date.now() / 1000);
    const blob = new Blob([JSON.stringify(recording, null, 2) + '\n'], {type: 'application/json'});
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'rehearsal.json';
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }
  function command(action, slideIndex){
    recordPoint(action, slideIndex);
    fetch('/api/command', {method:'POST', headers:{'content-type':'application/json'}, body: JSON.stringify({action, slideIndex})});
  }
  document.querySelectorAll('[data-cmd]').forEach(button => button.addEventListener('click', () => {
    const cmd = button.dataset.cmd;
    if (cmd === 'goto' && button.dataset.slideIndex) command('goto', parseInt(button.dataset.slideIndex || '0', 10));
    else if (cmd === 'goto') command('goto', Math.max(0, parseInt(document.getElementById('goto-input').value || '1', 10) - 1));
    else command(cmd, 0);
  }));
  document.querySelectorAll('[data-record]').forEach(button => button.addEventListener('click', () => {
    if (button.dataset.record === 'toggle') {
      const active = toggleRecording();
      button.textContent = active ? 'Stop Recording' : 'Record';
    }
    if (button.dataset.record === 'download') downloadRecording();
  }));
  const events = new EventSource('/events');
  events.addEventListener('state', event => { state = JSON.parse(event.data); render(); });
  setInterval(render, 500);
  render();
})();
`
}

func remoteJS() string {
	return `
(function(){
  const deck = window.__SLIDES_DECK__;
  let state = window.__SLIDES_STATE__ || {slideIndex:0, clickStep:0};
  const current = document.getElementById('remote-current');
  const meta = document.getElementById('remote-meta');
  const timer = document.getElementById('remote-timer');
  function elapsedSeconds(){
    if (!state.startedAtUnix) return 0;
    const now = Math.floor(Date.now() / 1000);
    const endpoint = state.paused && state.pausedAtUnix ? state.pausedAtUnix : now;
    return Math.max(0, endpoint - state.startedAtUnix - (state.pausedTotalSecs || 0));
  }
  function formatElapsed(total){
    const minutes = Math.floor(total / 60);
    const seconds = total % 60;
    return String(minutes).padStart(2, '0') + ':' + String(seconds).padStart(2, '0');
  }
  function render(){
    const slide = deck.slides[state.slideIndex] || deck.slides[0] || {};
    current.textContent = (state.slideIndex + 1) + '. ' + (slide.title || '');
    meta.textContent = 'Click ' + (state.clickStep || 0) + ' of ' + (slide.clicks || 0) + (state.paused ? ' / paused' : '');
    if (timer) timer.textContent = formatElapsed(elapsedSeconds());
  }
  function command(action, slideIndex){
    fetch('/api/command', {method:'POST', headers:{'content-type':'application/json'}, body: JSON.stringify({action, slideIndex})});
  }
  document.querySelectorAll('[data-cmd]').forEach(button => button.addEventListener('click', () => command(button.dataset.cmd, 0)));
  document.getElementById('remote-goto').addEventListener('submit', event => {
    event.preventDefault();
    const n = parseInt(new FormData(event.currentTarget).get('slide') || '1', 10);
    command('goto', Math.max(0, n - 1));
  });
  const events = new EventSource('/events');
  events.addEventListener('state', event => { state = JSON.parse(event.data); render(); });
  setInterval(render, 500);
  render();
})();
`
}
