package slides

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"sync"

	"m31labs.dev/gosx/server"
	"m31labs.dev/mdpp"
)

const maxSourceBytes = 1 << 20

func sourceRevision(src []byte) string { sum := sha256.Sum256(src); return hex.EncodeToString(sum[:]) }

// Literal loopback addresses and localhost are the trusted authoring authorities.
// Never trust a request hostname merely because it resolves to the listener.
func trustedSourceHost(authority string) bool {
	u, err := url.Parse("//" + authority)
	if err != nil || u.Host != authority || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
	default:
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		return err == nil && n > 0 && n <= 65535
	}
	return !strings.HasSuffix(authority, ":")
}

func authorizeSourceRequest(r *http.Request, token string, requireToken bool) error {
	if !sourceRequestWriter(r) {
		return fmt.Errorf("local authoring host or editor session required")
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return fmt.Errorf("same-origin authoring required")
	}
	origin := r.Header.Get("Origin")
	if origin != "" {
		u, err := url.Parse(origin)
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if err != nil || u.Scheme != scheme || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return fmt.Errorf("same-origin authoring required")
		}
	}
	if requireToken && (origin == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Slides-Token")), []byte(token)) != 1) {
		return fmt.Errorf("authoring token required")
	}
	return nil
}

// The token is returned only by a same-origin GET. Writes also require an
// Origin check and the current source revision, so another tab cannot silently
// overwrite edits. Export servers never mount this endpoint.
func mountSourceEditor(app *server.App, deck *IslandDeck) error {
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	token := hex.EncodeToString(random[:])
	path := filepath.Join(deck.Dir, DeckFileName)
	var mu sync.Mutex
	mountSceneEditor(app, deck, token, &mu)
	mountSourceTools(app, token)
	if err := mountProjectEditor(app, deck, token, &mu); err != nil {
		return err
	}
	app.Mount("/_slides/source", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		fail := func(status int, message string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			w.Header().Set("Allow", "GET, PUT")
			fail(405, "method not allowed")
			return
		}
		if err := authorizeSourceRequest(r, token, r.Method == http.MethodPut); err != nil {
			fail(403, err.Error())
			return
		}
		mu.Lock()
		defer mu.Unlock()
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			fail(400, "deck.md must be a regular file")
			return
		}
		if info.Size() > maxSourceBytes {
			fail(413, "deck.md exceeds the 1 MiB authoring limit")
			return
		}
		src, err := os.ReadFile(path)
		if err != nil {
			fail(500, "could not read deck.md")
			return
		}
		if r.Method == http.MethodGet {
			if r.URL.Query().Get("motion") == "1" {
				loaded, parseErr := parseIslandDeck(deck.Dir, src)
				if parseErr != nil {
					fail(422, parseErr.Error())
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"source": string(src), "revision": sourceRevision(src), "token": token, "motions": sourceMotionRanges(loaded)})
			} else {
				json.NewEncoder(w).Encode(map[string]string{"source": string(src), "revision": sourceRevision(src), "token": token})
			}
			return
		}
		var input struct {
			Source   string `json:"source"`
			Revision string `json:"revision"`
			Motions  []struct {
				Start int               `json:"start"`
				Attrs map[string]string `json:"attrs"`
			} `json:"motions"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6*maxSourceBytes+16384)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || len(input.Source) > maxSourceBytes {
			fail(400, "invalid source or authoring size limit exceeded")
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			fail(400, "expected one source edit")
			return
		}
		if input.Revision != sourceRevision(src) {
			fail(409, "deck.md changed; reload source before saving")
			return
		}
		if input.Motions != nil {
			if input.Source != "" || len(input.Motions) > 2000 {
				fail(400, "expected at most 2000 focused motion edits")
				return
			}
			doc, parseErr := mdpp.Parse(src)
			if parseErr != nil {
				fail(422, parseErr.Error())
				return
			}
			var edits []mdpp.SourceEdit
			seen := map[int]bool{}
			for _, motion := range input.Motions {
				if seen[motion.Start] {
					fail(400, "duplicate motion edit")
					return
				}
				seen[motion.Start] = true
				for key := range motion.Attrs {
					switch key {
					case "preset", "duration", "delay", "easing", "replay":
					default:
						fail(400, "unsupported motion attribute")
						return
					}
				}
				isMotion := false
				doc.Root.Walk(func(n *mdpp.Node) bool {
					if n.Type == mdpp.NodeContainerDirective && n.Range.StartByte == motion.Start && n.Attr("name") == "motion" {
						isMotion = true
					}
					return true
				})
				if !isMotion {
					fail(400, "motion edit must target a parsed motion directive")
					return
				}
				edit, err := mdpp.EditDirectiveAttributes(doc, motion.Start, motion.Attrs)
				if err != nil {
					fail(422, err.Error())
					return
				}
				edits = append(edits, edit)
			}
			sort.Slice(edits, func(i, j int) bool { return edits[i].Range.StartByte > edits[j].Range.StartByte })
			input.Source = string(src)
			for _, edit := range edits {
				input.Source = input.Source[:edit.Range.StartByte] + edit.NewText + input.Source[edit.Range.EndByte:]
			}
			if len(input.Source) > maxSourceBytes {
				fail(400, "authoring size limit exceeded")
				return
			}
		}
		fresh, err := parseIslandDeck(deck.Dir, []byte(input.Source))
		if err == nil && len(fresh.Slides) == 0 {
			err = fmt.Errorf("the deck needs at least one slide")
		}
		if err == nil {
			_, failures := fresh.compileComponents()
			if len(failures) > 0 {
				err = fmt.Errorf("component validation failed: %v", failures)
			}
		}
		if err == nil {
			_, err = compileDeckProgram(fresh)
		}
		if err != nil {
			fail(422, err.Error())
			return
		}
		current, err := os.ReadFile(path)
		if err != nil || sourceRevision(current) != input.Revision {
			fail(409, "deck.md changed during validation; reload source")
			return
		}
		save, err := stageSourceEdit(path, input.Source, info.Mode().Perm())
		if err != nil {
			fail(500, "could not prepare save")
			return
		}
		defer save.cleanup()
		err = save.capture(input.Revision)
		if err == nil {
			err = save.publish(input.Revision)
		}
		if err != nil {
			var conflict *sourceEditConflict
			if errors.As(err, &conflict) {
				fail(409, err.Error())
			} else {
				fail(500, err.Error())
			}
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"revision": sourceRevision([]byte(input.Source)), "message": "Saved deck.md"})
	}))
	return nil
}

// Keep the displaced inode, including edits from an already-open external file
// descriptor. Portable Go has no hash-conditional rename, so publishing uses a
// no-overwrite hard link after capture. Never discard a captured revision.
type stagedSourceEdit struct{ path, dir, prepared, previous string }

type sourceEditConflict struct{ recovery string }

func (e *sourceEditConflict) Error() string {
	return filepath.Base(e.recovery) + " changed during save; reload source. Previous source retained at " + e.recovery
}

func stageSourceEdit(path, source string, mode os.FileMode) (*stagedSourceEdit, error) {
	dir, err := os.MkdirTemp(filepath.Dir(path), ".slides-history-*")
	if err != nil {
		return nil, err
	}
	save := &stagedSourceEdit{path: path, dir: dir, prepared: filepath.Join(dir, "next.md"), previous: filepath.Join(dir, filepath.Base(path))}
	file, err := os.OpenFile(save.prepared, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err == nil {
		err = file.Chmod(mode)
		if err == nil {
			_, err = file.WriteString(source)
		}
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		save.cleanup()
		return nil, err
	}
	// Establish filesystem support before moving the author's current source.
	probe := filepath.Join(dir, "link-check")
	if err := os.Link(save.prepared, probe); err != nil {
		save.cleanup()
		return nil, err
	}
	os.Remove(probe)
	return save, nil
}

func (s *stagedSourceEdit) cleanup() {
	os.Remove(filepath.Join(s.dir, "link-check"))
	os.Remove(s.prepared)
	os.Remove(s.dir) // Removes only empty directories; retained revisions survive.
}

func (s *stagedSourceEdit) matches(revision string) bool {
	info, err := os.Lstat(s.previous)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	file, err := os.Open(s.previous)
	if err != nil {
		return false
	}
	defer file.Close()
	src, err := io.ReadAll(io.LimitReader(file, maxSourceBytes+1))
	return err == nil && len(src) <= maxSourceBytes && sourceRevision(src) == revision
}

func (s *stagedSourceEdit) conflict() error {
	return &sourceEditConflict{recovery: filepath.Join(filepath.Base(s.dir), filepath.Base(s.path))}
}

func (s *stagedSourceEdit) capture(revision string) error {
	if err := os.Rename(s.path, s.previous); err != nil {
		return fmt.Errorf("could not capture %s for save: %w", filepath.Base(s.path), err)
	}
	if !s.matches(revision) {
		os.Link(s.previous, s.path) // Restore only if another writer has not recreated it.
		return s.conflict()
	}
	return nil
}

func (s *stagedSourceEdit) publish(revision string) error {
	if err := os.Link(s.prepared, s.path); err != nil {
		os.Link(s.previous, s.path) // Fail closed; never overwrite to recover.
		if errors.Is(err, os.ErrExist) {
			return s.conflict()
		}
		return fmt.Errorf("could not publish %s; previous source retained at %s: %w", filepath.Base(s.path), filepath.Join(filepath.Base(s.dir), filepath.Base(s.path)), err)
	}
	if !s.matches(revision) {
		return s.conflict()
	}
	return nil
}
