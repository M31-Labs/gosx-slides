package slides

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"m31labs.dev/gosx/server"
)

// Only the .sir/Steps pair actually declared in deck.md is editable. Compiled
// SceneIR is never patched: regeneration preserves Sirena's attachments.
type sceneEditSource struct {
	ref                         ComponentRef
	props                       map[string]any
	source, steps               []byte
	path                        string
	mode                        os.FileMode
	contextRevision             string
	deckRevision, inputRevision string
}

func readSceneAuthorFile(dir, name string) ([]byte, os.FileMode, error) {
	if !safeDeckRelPath(name) {
		return nil, 0, fmt.Errorf("scene authoring requires a deck-relative file")
	}
	path := dir
	for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(name)), "/") {
		path = filepath.Join(path, part)
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, 0, fmt.Errorf("scene authoring requires regular files without symlinks")
		}
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSourceBytes {
		return nil, 0, fmt.Errorf("scene authoring requires regular files of at most 1 MiB")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, maxSourceBytes+1))
	if err != nil || len(raw) > maxSourceBytes {
		return nil, 0, fmt.Errorf("scene authoring size limit exceeded")
	}
	return raw, info.Mode().Perm(), nil
}

func loadSceneEditSource(dir, id string) (*sceneEditSource, error) {
	deckSource, _, err := readSceneAuthorFile(dir, DeckFileName)
	if err != nil {
		return nil, err
	}
	deck, err := parseIslandDeck(dir, deckSource)
	if err != nil {
		return nil, err
	}
	for _, ref := range deckGraphicRefs(deck) {
		if ref.Name != "Scene3D" || sceneGraphicID(ref) != id {
			continue
		}
		props := parseProps(ref.Props)
		src, steps := graphicString(props, "Src", ""), graphicString(props, "Steps", "")
		if !strings.EqualFold(filepath.Ext(src), ".sir") || steps == "" || filepath.Clean(src) == filepath.Clean(steps) || filepath.Clean(steps) == DeckFileName {
			return nil, fmt.Errorf("declare Scene3D Src=\"diagram.sir\" Steps=\"steps.json\" to edit scene cues")
		}
		source, _, err := readSceneAuthorFile(dir, src)
		if err != nil {
			return nil, err
		}
		stepSource, mode, err := readSceneAuthorFile(dir, steps)
		if err != nil {
			return nil, err
		}
		context := []string{sourceRevision(deckSource), sourceRevision(source)}
		if shader := graphicString(props, "Shader", ""); shader != "" {
			if filepath.Clean(shader) == filepath.Clean(steps) {
				return nil, fmt.Errorf("steps must use a separate file")
			}
			raw, _, err := readSceneAuthorFile(dir, shader)
			if err != nil {
				return nil, err
			}
			context = append(context, sourceRevision(raw))
		}
		raw, _ := json.Marshal(context)
		inputs, _ := json.Marshal(context[1:])
		return &sceneEditSource{ref: ref, props: props, source: source, steps: stepSource, path: filepath.Join(dir, filepath.FromSlash(steps)), mode: mode, contextRevision: sourceRevision(raw), deckRevision: context[0], inputRevision: sourceRevision(inputs)}, nil
	}
	return nil, fmt.Errorf("scene is no longer declared in deck.md; reload the deck")
}

func mountSceneEditor(app *server.App, deck *IslandDeck, token string, mu *sync.Mutex) {
	app.Mount("/_slides/scene", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		fail := func(status int, message string) {
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
		if r.Method != "GET" && r.Method != "POST" && r.Method != "PUT" {
			w.Header().Set("Allow", "GET, POST, PUT")
			fail(405, "method not allowed")
			return
		}
		if !trustedSourceHost(r.Host) {
			fail(403, "local authoring host required")
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
			if err != nil || u.Scheme != scheme || u.Host != r.Host || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				fail(403, "same-origin authoring required")
				return
			}
		}
		if r.Method != "GET" && (origin == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Slides-Token")), []byte(token)) != 1) {
			fail(403, "authoring token required")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		current, err := loadSceneEditSource(deck.Dir, r.URL.Query().Get("graphic"))
		if err != nil {
			fail(400, err.Error())
			return
		}
		steps := current.steps
		var input struct {
			Source          string `json:"source"`
			Revision        string `json:"revision"`
			ContextRevision string `json:"contextRevision"`
		}
		if r.Method != "GET" {
			r.Body = http.MaxBytesReader(w, r.Body, 6*maxSourceBytes+1024)
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&input); err != nil || len(input.Source) > maxSourceBytes {
				fail(400, "invalid scene edit or size limit exceeded")
				return
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				fail(400, "expected one scene edit")
				return
			}
			if input.Revision != sourceRevision(current.steps) || input.ContextRevision != current.contextRevision {
				fail(409, "Scene source changed; reload the deck before saving cues.")
				return
			}
			steps = []byte(input.Source)
		}
		payload, err := compileSirenaGraphic(deck.Dir, current.props, current.source, steps)
		if err != nil {
			fail(422, err.Error())
			return
		}
		timeline := payload["slideSteps"]
		if _, err := graphicStepAttrs(payload); err != nil {
			fail(422, err.Error())
			return
		}
		if r.Method == "PUT" {
			latest, err := loadSceneEditSource(deck.Dir, sceneGraphicID(current.ref))
			if err != nil || latest.contextRevision != current.contextRevision || !bytes.Equal(latest.steps, current.steps) {
				fail(409, "Scene source changed during validation; reload the deck.")
				return
			}
			save, err := stageSourceEdit(current.path, input.Source, current.mode)
			if err != nil {
				fail(500, "could not prepare scene save")
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
		}
		json.NewEncoder(w).Encode(map[string]any{"source": string(steps), "revision": sourceRevision(steps), "contextRevision": current.contextRevision, "deckRevision": current.deckRevision, "inputRevision": current.inputRevision, "token": token, "timeline": timeline, "scene": payload["scene"], "camera": payload["camera"], "file": graphicString(current.props, "Steps", "")})
	}))
}
