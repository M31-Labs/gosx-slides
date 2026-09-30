package slides

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"sync"

	"m31labs.dev/gosx/server"
)

const maxSourceBytes = 1 << 20

func sourceRevision(src []byte) string { sum := sha256.Sum256(src); return hex.EncodeToString(sum[:]) }

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
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			fail(403, "same-origin authoring required")
			return
		}
		origin := r.Header.Get("Origin")
		if origin != "" {
			u, err := url.Parse(origin)
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			if err != nil || u.Scheme != scheme || u.Host != r.Host || u.Path != "" {
				fail(403, "same-origin authoring required")
				return
			}
		}
		if r.Method == http.MethodPut && (origin == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Slides-Token")), []byte(token)) != 1) {
			fail(403, "authoring token required")
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
			json.NewEncoder(w).Encode(map[string]string{"source": string(src), "revision": sourceRevision(src), "token": token})
			return
		}
		var input struct {
			Source   string `json:"source"`
			Revision string `json:"revision"`
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
		file, err := os.CreateTemp(deck.Dir, ".slides-edit-*")
		if err != nil {
			fail(500, "could not prepare save")
			return
		}
		name := file.Name()
		defer os.Remove(name)
		if err = file.Chmod(info.Mode().Perm()); err == nil {
			_, err = file.WriteString(input.Source)
		}
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(name, path)
		}
		if err != nil {
			fail(500, "could not save deck.md")
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"revision": sourceRevision([]byte(input.Source)), "message": "Saved deck.md"})
	}))
	return nil
}
