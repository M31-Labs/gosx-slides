package slides

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"m31labs.dev/gosx/crdt"
	"m31labs.dev/gosx/hub"
	"m31labs.dev/gosx/server"
)

const teamTextLimit = 128 << 10
const teamEditLimit = 16 << 10
const teamSnapshotLimit = 16 << 20
const teamFileLimit = 24 << 20
const teamStateFile = ".slides-team.json"

type teamSnapshot struct {
	Revision string `json:"revision"`
	Data     []byte `json:"data"`
}
type teamComment struct {
	ID       string    `json:"id"`
	Slide    string    `json:"slide,omitempty"`
	Cue      string    `json:"cue,omitempty"`
	Quote    string    `json:"quote,omitempty"`
	Text     string    `json:"text"`
	Label    string    `json:"label"`
	Resolved bool      `json:"resolved"`
	Created  time.Time `json:"created"`
}
type teamStored struct {
	Version      int            `json:"version"`
	TextID       crdt.ObjID     `json:"textID"`
	Revision     string         `json:"revision"`
	DiskRevision string         `json:"diskRevision"`
	History      []teamSnapshot `json:"history"`
	Comments     []teamComment  `json:"comments"`
}
type teamMember struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Writer bool   `json:"writer"`
	Slide  int    `json:"slide"`
	Step   int    `json:"step"`
}
type teamEdit struct {
	ID     string `json:"id"`
	Base   string `json:"base"`
	Index  uint64 `json:"index"`
	Delete uint64 `json:"delete"`
	Insert string `json:"insert"`
}
type teamRoom struct {
	mu           sync.Mutex
	dir          string
	doc          *crdt.Doc
	stored       teamStored
	hub          *hub.Hub
	members      map[string]teamMember
	publisher    string
	publishUntil time.Time
}

func teamNewDocument(text string) (*crdt.Doc, crdt.ObjID, []byte, error) {
	doc, err := crdt.NewDocChecked()
	if err != nil {
		return nil, "", nil, err
	}
	id, err := doc.MakeText(crdt.Root, "source")
	if err == nil {
		_, _, err = doc.SpliceText(id, 0, 0, text)
	}
	if err == nil {
		_, err = doc.Commit("Shared presentation draft")
	}
	if err != nil {
		return nil, "", nil, err
	}
	data, err := doc.Save()
	return doc, id, data, err
}

func newTeamRoom(deck *IslandDeck) (*teamRoom, error) {
	if len(deck.Source) > teamTextLimit || !utf8.Valid(deck.Source) {
		return nil, fmt.Errorf("collaboration requires UTF-8 deck.md up to 128 KiB")
	}
	room := &teamRoom{dir: deck.Dir, members: map[string]teamMember{}}
	root, err := os.OpenRoot(deck.Dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(teamStateFile)
	if err == nil {
		defer file.Close()
		info, statErr := file.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > teamFileLimit {
			return nil, fmt.Errorf("invalid collaboration state file")
		}
		data, readErr := io.ReadAll(io.LimitReader(file, teamFileLimit+1))
		if readErr != nil || len(data) > teamFileLimit {
			return nil, fmt.Errorf("collaboration state exceeds limit")
		}
		if err = json.Unmarshal(data, &room.stored); err != nil {
			return nil, fmt.Errorf("read collaboration state: %w", err)
		}
		if room.stored.Version != 1 || len(room.stored.History) == 0 || len(room.stored.History) > 8 || len(room.stored.Comments) > 200 {
			return nil, fmt.Errorf("invalid collaboration state version/history")
		}
		total := 0
		for _, snapshot := range room.stored.History {
			total += len(snapshot.Data)
			if snapshot.Revision != sourceRevision(snapshot.Data) {
				return nil, fmt.Errorf("invalid collaboration snapshot revision")
			}
		}
		if total > teamSnapshotLimit {
			return nil, fmt.Errorf("collaboration snapshot budget exceeded")
		}
		last := room.stored.History[len(room.stored.History)-1]
		if last.Revision != room.stored.Revision {
			return nil, fmt.Errorf("collaboration current revision missing")
		}
		room.doc, err = crdt.Load(last.Data)
		if err != nil {
			return nil, fmt.Errorf("load collaboration CRDT: %w", err)
		}
		text, err := room.doc.TextToString(room.stored.TextID)
		if err != nil || len(text) > teamTextLimit || !utf8.ValidString(text) {
			return nil, fmt.Errorf("invalid collaboration draft text")
		}
		// External disk edits remain a visible publication conflict. The draft is
		// never replaced merely because an editor restarted the server.
		return room, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var deckSnapshot []byte
	room.doc, room.stored.TextID, deckSnapshot, err = teamNewDocument(string(deck.Source))
	if err != nil {
		return nil, err
	}
	room.stored.Version = 1
	room.stored.Revision = sourceRevision(deckSnapshot)
	room.stored.DiskRevision = sourceRevision(deck.Source)
	room.stored.History = []teamSnapshot{{room.stored.Revision, deckSnapshot}}
	if err = room.persist(room.stored); err != nil {
		return nil, err
	}
	return room, nil
}

func (r *teamRoom) persist(state teamStored) error {
	data, err := json.Marshal(state)
	if err != nil || len(data) > teamFileLimit {
		return fmt.Errorf("collaboration state budget exceeded")
	}
	root, err := os.OpenRoot(r.dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	name := ".slides-team-" + hex.EncodeToString(random[:]) + ".tmp"
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(name)
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = root.Rename(name, teamStateFile)
	}
	return err
}

func (r *teamRoom) text() string { text, _ := r.doc.TextToString(r.stored.TextID); return text }
func (r *teamRoom) publishing() bool {
	if r.publisher != "" && time.Now().After(r.publishUntil) {
		r.publisher = ""
	}
	return r.publisher != ""
}
func teamWriter(client *hub.Client) bool { value, _ := client.Metadata("writer"); return value == "1" }
func (r *teamRoom) sendState(client *hub.Client, ack map[string]string) {
	// Audience/viewer connections never receive a draft, notes, source quotes,
	// or comments. Collaboration is an explicit authoring surface.
	if !teamWriter(client) {
		r.hub.Send(client.ID, "team:error", map[string]string{"message": "Shared drafts require an editor session."})
		return
	}
	r.hub.Send(client.ID, "team:state", map[string]any{"text": r.text(), "revision": r.stored.Revision, "diskRevision": r.stored.DiskRevision, "comments": r.stored.Comments, "publishing": r.publishing(), "ack": ack})
}
func (r *teamRoom) broadcastState(ack map[string]string) {
	r.hub.BroadcastWhere("team:state", map[string]any{"text": r.text(), "revision": r.stored.Revision, "diskRevision": r.stored.DiskRevision, "comments": r.stored.Comments, "publishing": r.publishing(), "ack": ack}, teamWriter)
}
func (r *teamRoom) broadcastPresence() {
	members := make([]teamMember, 0, len(r.members))
	for _, member := range r.members {
		members = append(members, member)
	}
	r.hub.BroadcastWhere("team:presence", members, teamWriter)
}
func (r *teamRoom) fail(ctx *hub.Context, id string, err error, retry bool) {
	r.hub.Send(ctx.Client.ID, "team:error", map[string]any{"id": id, "message": err.Error(), "retry": retry})
}

func (r *teamRoom) splice(edit teamEdit) (string, error) {
	if len(edit.ID) == 0 || len(edit.ID) > 80 || len(edit.Insert) > teamEditLimit || !utf8.ValidString(edit.Insert) {
		return "", fmt.Errorf("edit needs an ID and UTF-8 insertion up to 16 KiB")
	}
	var snapshot []byte
	for _, base := range r.stored.History {
		if base.Revision == edit.Base {
			snapshot = base.Data
			break
		}
	}
	if snapshot == nil {
		return "", fmt.Errorf("base revision expired; your local draft is preserved, rebase before retrying")
	}
	base, err := crdt.Load(snapshot)
	if err != nil {
		return "", err
	}
	text, err := base.TextToString(r.stored.TextID)
	if err != nil {
		return "", err
	}
	runes := []rune(text)
	if edit.Index > uint64(len(runes)) || edit.Delete > uint64(len(runes))-edit.Index || len(string(runes[edit.Index:edit.Index+edit.Delete])) > teamEditLimit {
		return "", fmt.Errorf("edit range is invalid or deletion exceeds 16 KiB")
	}
	// A fresh actor is essential. Fork/Load retains the original actor and
	// would assign colliding operation IDs to simultaneous client splices.
	branch, err := crdt.NewDocChecked()
	if err == nil {
		err = branch.Merge(base)
	}
	if err == nil {
		_, _, err = branch.SpliceText(r.stored.TextID, edit.Index, edit.Delete, edit.Insert)
	}
	if err == nil {
		_, err = branch.Commit("Shared draft edit")
	}
	if err != nil {
		return "", err
	}
	branchData, err := branch.Save()
	if err != nil {
		return "", err
	}
	candidate, err := crdt.NewDocChecked()
	if err == nil {
		err = candidate.Merge(r.doc)
	}
	if err == nil {
		err = candidate.Merge(branch)
	}
	if err != nil {
		return "", err
	}
	merged, err := candidate.TextToString(r.stored.TextID)
	if err != nil || len(merged) > teamTextLimit {
		return "", fmt.Errorf("shared draft exceeds 128 KiB; your local draft is preserved")
	}
	data, err := candidate.Save()
	if err != nil {
		return "", err
	}
	branchRevision := sourceRevision(branchData)
	state := r.stored
	state.Revision = sourceRevision(data)
	state.History = append(append([]teamSnapshot{}, state.History...), teamSnapshot{branchRevision, branchData})
	if branchRevision != state.Revision {
		state.History = append(state.History, teamSnapshot{state.Revision, data})
	}
	total := 0
	for _, snap := range state.History {
		total += len(snap.Data)
	}
	for len(state.History) > 8 || total > teamSnapshotLimit {
		if len(state.History) <= 2 {
			return "", fmt.Errorf("CRDT snapshot budget reached; publish the shared draft to compact it")
		}
		total -= len(state.History[0].Data)
		state.History = state.History[1:]
	}
	if err = r.persist(state); err != nil {
		return "", fmt.Errorf("persist shared edit: %w", err)
	}
	r.doc, r.stored = candidate, state
	return branchRevision, nil
}

func (r *teamRoom) addComment(input teamComment, label string) error {
	input.Text, input.Quote = strings.TrimSpace(input.Text), strings.TrimSpace(input.Quote)
	if input.Text == "" || len(input.Text) > 4096 || len(input.Quote) > 512 || !utf8.ValidString(input.Text+input.Quote) || len(input.Slide) > 80 || len(input.Cue) > 80 || (input.Slide == "" && input.Quote == "") {
		return fmt.Errorf("comment needs a slide/cue or source quote anchor and text up to 4 KiB")
	}
	if (input.Slide != "" && !cueNamePattern.MatchString(input.Slide)) || (input.Cue != "" && (input.Slide == "" || !cueNamePattern.MatchString(input.Cue))) || (input.Quote != "" && !strings.Contains(r.text(), input.Quote)) {
		return fmt.Errorf("comment anchor is invalid or the source quote changed; your comment draft is preserved")
	}
	if len(r.stored.Comments) >= 200 {
		return fmt.Errorf("review supports at most 200 comments")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return err
	}
	input.ID, input.Label, input.Created, input.Resolved = hex.EncodeToString(id[:]), label, time.Now().UTC(), false
	state := r.stored
	state.Comments = append(append([]teamComment{}, state.Comments...), input)
	if err := r.persist(state); err != nil {
		return err
	}
	r.stored = state
	return nil
}
func (r *teamRoom) resolveComment(id string, resolved bool) error {
	state := r.stored
	state.Comments = append([]teamComment{}, state.Comments...)
	for i := range state.Comments {
		if state.Comments[i].ID == id {
			state.Comments[i].Resolved = resolved
			if err := r.persist(state); err != nil {
				return err
			}
			r.stored = state
			return nil
		}
	}
	return fmt.Errorf("review comment not found")
}

func (r *teamRoom) published() error {
	root, err := os.OpenRoot(r.dir)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.Open(DeckFileName)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("deck.md must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, teamTextLimit+1))
	if err != nil || len(data) > teamTextLimit || string(data) != r.text() {
		return fmt.Errorf("disk differs from the current shared draft; publication baseline was not changed")
	}
	state := r.stored
	state.DiskRevision = sourceRevision(data)
	doc := r.doc
	// Retain normal bases so typing buffered during publication can resume.
	// Large histories compact only at an explicit publication boundary; a
	// queued old-base edit then receives the ordinary preserve/rebase error.
	if len(state.History[len(state.History)-1].Data) >= teamSnapshotLimit/4 {
		var id crdt.ObjID
		var snapshot []byte
		doc, id, snapshot, err = teamNewDocument(string(data))
		if err != nil {
			return err
		}
		state.TextID, state.Revision = id, sourceRevision(snapshot)
		state.History = []teamSnapshot{{state.Revision, snapshot}}
	}
	if err := r.persist(state); err != nil {
		return err
	}
	r.doc, r.stored = doc, state
	return nil
}

func wireTeamHub(r *teamRoom) *hub.Hub {
	h := hub.New("slides-team")
	r.hub = h
	h.RequireOrigin, h.MaxClients, h.MaxMessagesPerSecond, h.MaxMessageBurst, h.MaxSyncMessageSize = true, 8, 12, 24, 64<<10
	h.SetBinaryAuthorizer(func(*hub.Client, string) bool { return false })
	h.On("join", func(ctx *hub.Context) { r.mu.Lock(); defer r.mu.Unlock(); r.sendState(ctx.Client, nil) })
	h.On("leave", func(ctx *hub.Context) {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.members, ctx.Client.ID)
		if r.publisher == ctx.Client.ID {
			r.publisher = ""
			r.broadcastState(nil)
		}
		r.broadcastPresence()
	})
	h.On("team:sync", func(ctx *hub.Context) { r.mu.Lock(); defer r.mu.Unlock(); r.sendState(ctx.Client, nil) })
	h.On("team:presence", func(ctx *hub.Context) {
		if !teamWriter(ctx.Client) {
			return
		}
		var member teamMember
		if json.Unmarshal(ctx.Data, &member) != nil || len(member.Label) > 64 || !utf8.ValidString(member.Label) || member.Slide < 0 || member.Slide > 10000 || member.Step < 0 || member.Step > 10000 {
			return
		}
		member.ID, member.Writer = ctx.Client.ID, true
		member.Label = strings.TrimSpace(member.Label)
		if member.Label == "" {
			member.Label = "Editor"
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		r.members[member.ID] = member
		r.broadcastPresence()
	})
	h.On("team:edit", func(ctx *hub.Context) {
		var edit teamEdit
		if json.Unmarshal(ctx.Data, &edit) != nil {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if !teamWriter(ctx.Client) {
			r.fail(ctx, edit.ID, fmt.Errorf("editor role required"), false)
			return
		}
		if r.publishing() {
			r.fail(ctx, edit.ID, fmt.Errorf("publication is in progress; your edit will retry after it finishes"), true)
			return
		}
		branch, err := r.splice(edit)
		if err != nil {
			r.fail(ctx, edit.ID, err, false)
			return
		}
		r.broadcastState(map[string]string{"id": edit.ID, "client": ctx.Client.ID, "branch": branch})
	})
	h.On("team:comment", func(ctx *hub.Context) {
		if !teamWriter(ctx.Client) {
			r.fail(ctx, "", fmt.Errorf("editor role required"), false)
			return
		}
		var comment teamComment
		if json.Unmarshal(ctx.Data, &comment) != nil {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		label := r.members[ctx.Client.ID].Label
		if label == "" {
			label = "Editor"
		}
		if err := r.addComment(comment, label); err != nil {
			r.fail(ctx, "", err, false)
			return
		}
		r.broadcastState(nil)
		h.Send(ctx.Client.ID, "team:comment-saved", map[string]string{"text": strings.TrimSpace(comment.Text)})
	})
	h.On("team:resolve", func(ctx *hub.Context) {
		if !teamWriter(ctx.Client) {
			r.fail(ctx, "", fmt.Errorf("editor role required"), false)
			return
		}
		var input struct {
			ID       string `json:"id"`
			Resolved bool   `json:"resolved"`
		}
		if json.Unmarshal(ctx.Data, &input) != nil {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if err := r.resolveComment(input.ID, input.Resolved); err != nil {
			r.fail(ctx, "", err, false)
			return
		}
		r.broadcastState(nil)
	})
	h.On("team:publish", func(ctx *hub.Context) {
		if !teamWriter(ctx.Client) {
			r.fail(ctx, "", fmt.Errorf("editor role required"), false)
			return
		}
		var input struct {
			Phase    string `json:"phase"`
			Revision string `json:"revision"`
		}
		if json.Unmarshal(ctx.Data, &input) != nil {
			return
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if input.Phase == "begin" {
			if r.publishing() || input.Revision != r.stored.Revision {
				r.fail(ctx, "", fmt.Errorf("shared draft changed or another editor is publishing; retry"), false)
				return
			}
			r.publisher, r.publishUntil = ctx.Client.ID, time.Now().Add(20*time.Second)
			r.broadcastState(nil)
			h.Send(ctx.Client.ID, "team:publish-ready", map[string]string{"text": r.text(), "diskRevision": r.stored.DiskRevision})
			return
		}
		active := r.publishing()
		if active && r.publisher != ctx.Client.ID {
			r.fail(ctx, "", fmt.Errorf("publication lease expired; verify the disk and shared draft"), false)
			return
		}
		r.publisher = ""
		if input.Phase == "finish" {
			if err := r.published(); err != nil {
				r.fail(ctx, "", err, false)
			}
		}
		r.broadcastState(nil)
	})
	return h
}

func mountTeam(app *server.App, deck *IslandDeck) error {
	r, err := newTeamRoom(deck)
	if err != nil {
		return err
	}
	h := wireTeamHub(r)
	app.Mount("/_slides/team", http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if request.Header.Get("Origin") == "" {
			http.Error(w, "browser Origin required", 403)
			return
		}
		if err := authorizeSourceRequest(request, "", false); err != nil {
			http.Error(w, err.Error(), 403)
			return
		}
		writer := sourceRequestWriter(request)
		if !writer {
			http.Error(w, "editor session required", http.StatusForbidden)
			return
		}
		flag := "0"
		if writer {
			flag = "1"
		}
		h.ServeHTTPWithMetadata(w, request, hub.ConnectionMetadata{"writer": flag})
	}))
	return nil
}
