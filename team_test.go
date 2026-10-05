package slides

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/net/websocket"
	"m31labs.dev/gosx/hub"
)

func testTeamRoom(t *testing.T, text string) *teamRoom {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := newTeamRoom(&IslandDeck{Dir: dir, Source: []byte(text)})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTeamConcurrentUnicodeAndSamePositionSplices(t *testing.T) {
	r := testTeamRoom(t, "A🙂BCéD")
	base := r.stored.Revision
	edits := []teamEdit{{ID: "first", Base: base, Index: 2, Delete: 1, Insert: "β"}, {ID: "second", Base: base, Index: 4, Delete: 1, Insert: "中"}}
	var wait sync.WaitGroup
	for _, edit := range edits {
		wait.Add(1)
		go func(edit teamEdit) {
			defer wait.Done()
			r.mu.Lock()
			defer r.mu.Unlock()
			if _, err := r.splice(edit); err != nil {
				t.Error(err)
			}
		}(edit)
	}
	wait.Wait()
	if r.text() != "A🙂βC中D" {
		t.Fatalf("disjoint Unicode edits lost text: %q", r.text())
	}
	base = r.stored.Revision
	if _, err := r.splice(teamEdit{ID: "x", Base: base, Index: 2, Insert: "X"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.splice(teamEdit{ID: "y", Base: base, Index: 2, Insert: "Y"}); err != nil {
		t.Fatal(err)
	}
	if text := r.text(); strings.Count(text, "X") != 1 || strings.Count(text, "Y") != 1 || !utf8.ValidString(text) || !strings.Contains(text, "βC中D") {
		t.Fatalf("same-position edits failed: %q", text)
	}
}

func TestTeamAcknowledgedBranchSupportsBufferedTyping(t *testing.T) {
	r := testTeamRoom(t, "ABC")
	base := r.stored.Revision
	if _, err := r.splice(teamEdit{ID: "remote", Base: base, Index: 3, Insert: "R"}); err != nil {
		t.Fatal(err)
	}
	branch, err := r.splice(teamEdit{ID: "local", Base: base, Index: 0, Insert: "L"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.splice(teamEdit{ID: "buffered", Base: branch, Index: 1, Insert: "🙂"}); err != nil {
		t.Fatal(err)
	}
	if r.text() != "L🙂ABCR" {
		t.Fatal("buffered typing did not retain remote edit", r.text())
	}
}

func TestTeamPersistenceFailureDoesNotAcknowledgeOrChangeDraft(t *testing.T) {
	r := testTeamRoom(t, "Private draft")
	before, revision := r.text(), r.stored.Revision
	path := filepath.Join(r.dir, teamStateFile)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := r.splice(teamEdit{ID: "cannot-save", Base: revision, Insert: "Missing"}); err == nil {
		t.Fatal("unpersisted edit acknowledged")
	}
	if r.text() != before || r.stored.Revision != revision {
		t.Fatal("failed persistence changed authoritative draft")
	}
	if paths, _ := filepath.Glob(filepath.Join(r.dir, ".slides-team-*.tmp")); len(paths) > 0 {
		t.Fatal("failed save leaked temporary state")
	}
}

func TestTeamPersistenceCommentsHistoryAndPublication(t *testing.T) {
	r := testTeamRoom(t, "# Draft\n")
	first := r.stored.Revision
	for i := 0; i < 12; i++ {
		if _, err := r.splice(teamEdit{ID: "edit", Base: r.stored.Revision, Index: uint64(utf8.RuneCountInString(r.text())), Insert: "🙂"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(r.stored.History) > 8 {
		t.Fatal("history not bounded")
	}
	text := r.text()
	if _, err := r.splice(teamEdit{ID: "stale", Base: first, Insert: "lost"}); err == nil || !strings.Contains(err.Error(), "preserved") || r.text() != text {
		t.Fatal("stale base changed shared draft", err)
	}
	if _, err := r.splice(teamEdit{ID: "large", Base: r.stored.Revision, Insert: strings.Repeat("x", teamEditLimit+1)}); err == nil {
		t.Fatal("large edit accepted")
	}
	if err := r.addComment(teamComment{Slide: "opening", Cue: "evidence", Text: "Review <this> literally", Resolved: true}, "Chosen label"); err != nil {
		t.Fatal(err)
	}
	id := r.stored.Comments[0].ID
	if r.stored.Comments[0].Resolved {
		t.Fatal("comment input resolved itself")
	}
	if err := r.resolveComment(id, true); err != nil {
		t.Fatal(err)
	}
	loaded, err := newTeamRoom(&IslandDeck{Dir: r.dir, Source: []byte("# Draft\n")})
	if err != nil || loaded.text() != text || len(loaded.stored.Comments) != 1 || !loaded.stored.Comments[0].Resolved || loaded.stored.Comments[0].ID != id {
		t.Fatalf("restart lost draft/review: %v", err)
	}
	if err := loaded.resolveComment(id, false); err != nil {
		t.Fatal(err)
	}
	originalDisk := loaded.stored.DiskRevision
	if err := loaded.published(); err == nil || loaded.stored.DiskRevision != originalDisk {
		t.Fatal("publication baseline changed without matching disk")
	}
	if err := os.WriteFile(filepath.Join(r.dir, DeckFileName), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if err := loaded.published(); err != nil {
		t.Fatal(err)
	}
	if len(loaded.stored.History) > 8 || loaded.stored.DiskRevision != sourceRevision([]byte(text)) {
		t.Fatal("publication did not reset baseline")
	}
	if _, err := loaded.splice(teamEdit{ID: "after-publish", Base: loaded.stored.Revision, Insert: "Buffered"}); err != nil {
		t.Fatal("publication invalidated buffered typing", err)
	}
	info, err := os.Stat(filepath.Join(r.dir, teamStateFile))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("private state file permissions", err)
	}
}

func teamRead(t *testing.T, conn *websocket.Conn, event string) json.RawMessage {
	t.Helper()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	for i := 0; i < 30; i++ {
		var msg hub.Message
		if err := websocket.JSON.Receive(conn, &msg); err != nil {
			t.Fatal(err)
		}
		if msg.Event == event {
			return msg.Data
		}
	}
	t.Fatal("team event missing", event)
	return nil
}
func teamSend(t *testing.T, conn *websocket.Conn, event string, data any) {
	t.Helper()
	if err := websocket.JSON.Send(conn, map[string]any{"event": event, "data": data}); err != nil {
		t.Fatal(err)
	}
}

func TestTeamHubRoleMetadataOriginAndReconnect(t *testing.T) {
	r := testTeamRoom(t, "# Author only\n\n<!-- Private notes -->\n")
	h := wireTeamHub(r)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		writer := "1"
		if req.URL.Query().Get("viewer") == "1" {
			writer = "0"
		}
		h.ServeHTTPWithMetadata(w, req, hub.ConnectionMetadata{"writer": writer})
	}))
	defer server.Close()
	dial := func(path, origin string) *websocket.Conn {
		t.Helper()
		config, err := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+path, origin)
		if err != nil {
			t.Fatal(err)
		}
		conn, err := websocket.DialConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		conn.MaxPayloadBytes = 512 << 10
		return conn
	}
	writer := dial("/", server.URL)
	defer writer.Close()
	data := teamRead(t, writer, "team:state")
	if !strings.Contains(string(data), "Private notes") {
		t.Fatal("authorized writer missing explicit draft")
	}
	viewer := dial("/?viewer=1", server.URL)
	defer viewer.Close()
	if data := teamRead(t, viewer, "team:error"); strings.Contains(string(data), "Private notes") {
		t.Fatal("viewer source disclosure")
	}
	teamSend(t, viewer, "team:edit", map[string]any{"id": "forged", "base": r.stored.Revision, "insert": "forged", "writer": true})
	if data := teamRead(t, viewer, "team:error"); !strings.Contains(string(data), "editor role") {
		t.Fatal("forged writer accepted", string(data))
	}
	base := r.stored.Revision
	teamSend(t, writer, "team:edit", teamEdit{ID: "real", Base: base, Insert: "🙂"})
	teamRead(t, writer, "team:state")
	reconnected := dial("/", server.URL)
	defer reconnected.Close()
	if data := teamRead(t, reconnected, "team:state"); !strings.Contains(string(data), "🙂") {
		t.Fatal("reconnect lost shared edit")
	}
	config, _ := websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+"/", "http://evil.example")
	if conn, err := websocket.DialConfig(config); err == nil {
		conn.Close()
		t.Fatal("cross-origin websocket accepted")
	}
	if h.MaxClients != 8 || h.MaxMessagesPerSecond <= 0 || !h.RequireOrigin || h.MaxSyncMessageSize != 64<<10 {
		t.Fatal("hub not bounded")
	}
	for i := 0; i < 5; i++ {
		conn := dial("/", server.URL)
		defer conn.Close()
		teamRead(t, conn, "team:state")
	}
	config, _ = websocket.NewConfig("ws"+strings.TrimPrefix(server.URL, "http")+"/", server.URL)
	if conn, err := websocket.DialConfig(config); err == nil {
		conn.Close()
		t.Fatal("ninth collaboration client accepted")
	}
	if err := websocket.Message.Send(reconnected, strings.Repeat("x", 64<<10+1)); err != nil {
		t.Fatal(err)
	}
	var message string
	if err := websocket.Message.Receive(reconnected, &message); err == nil {
		t.Fatal("oversized frame did not close connection")
	}
}

func TestTeamModeAndOriginAuthorization(t *testing.T) {
	deck := loadDeckFromSource(t, "# Team\n", nil)
	for _, opts := range []ServeOptions{{Collaborate: true}, {Collaborate: true, Edit: true, Static: true}} {
		if _, err := deck.NewServer(opts); err == nil {
			t.Fatal("collaboration exposed outside editing/live mode")
		}
	}
	app, err := deck.NewServer(ServeOptions{Edit: true, Collaborate: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", "http://evil.example"} {
		req := httptest.NewRequest("GET", "http://localhost/_slides/team", nil)
		req.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		app.Build().ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal("team origin accepted", origin, w.Code)
		}
	}
}
