package slides

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"time"
)

const sessionGrantFile = ".slides-sessions.json"
const sessionGrantLimit = 256
const sessionGrantFileLimit = 128 << 10
const presentationSessionAge = 8 * time.Hour

type sessionGrant struct {
	Role    string    `json:"role"`
	Expires time.Time `json:"expires"`
}
type sessionGrantState struct {
	Version int                     `json:"version"`
	Scope   string                  `json:"scope"`
	Grants  map[string]sessionGrant `json:"grants"`
}
type sessionGrantWatch struct {
	timer *time.Timer
	close func()
}

// Positive grants make encrypted cookies revocable without changing GoSX's
// cookie, encryption or CSRF contract. Only active room logins are retained.
type sessionGrants struct {
	mu         sync.Mutex
	dir, scope string
	grants     map[string]sessionGrant
	watches    map[string]map[string]sessionGrantWatch
	failed     bool
}

func newSessionGrants(dir, scope string) (*sessionGrants, error) {
	grants := &sessionGrants{dir: dir, scope: scope, grants: map[string]sessionGrant{}, watches: map[string]map[string]sessionGrantWatch{}}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(sessionGrantFile)
	if errors.Is(err, os.ErrNotExist) {
		return grants, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > sessionGrantFileLimit {
		return nil, fmt.Errorf("session grants must be a regular private file below 128 KiB")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("session grants require private file permissions (0600)")
	}
	file, err := root.Open(sessionGrantFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, sessionGrantFileLimit+1))
	if err != nil || len(data) > sessionGrantFileLimit {
		return nil, fmt.Errorf("session grant file exceeds 128 KiB")
	}
	var state sessionGrantState
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&state); err != nil {
		return nil, fmt.Errorf("read session grants: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || state.Version != 1 || len(state.Grants) > sessionGrantLimit {
		return nil, fmt.Errorf("invalid session grant version/budget")
	}
	// A new process key or rotated room token invalidates previous room grants.
	if state.Scope != scope {
		return grants, nil
	}
	for id, grant := range state.Grants {
		decoded, err := hex.DecodeString(id)
		if err != nil || len(decoded) != 32 || (grant.Role != "editor" && grant.Role != "audience") || grant.Expires.IsZero() {
			return nil, fmt.Errorf("invalid session grant")
		}
		if grant.Expires.After(time.Now()) {
			grants.grants[id] = grant
		}
	}
	return grants, nil
}

func (g *sessionGrants) persist(grants map[string]sessionGrant) error {
	data, err := json.Marshal(sessionGrantState{Version: 1, Scope: g.scope, Grants: grants})
	if err != nil || len(data) > sessionGrantFileLimit {
		return fmt.Errorf("session grant budget exceeded")
	}
	root, err := os.OpenRoot(g.dir)
	if err != nil {
		return err
	}
	defer root.Close()
	var random [12]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	name := ".slides-session-" + hex.EncodeToString(random[:]) + ".tmp"
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
		err = root.Rename(name, sessionGrantFile)
	}
	return err
}

func (g *sessionGrants) valid(id, role string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	grant, ok := g.grants[id]
	return !g.failed && ok && grant.Role == role && grant.Expires.After(time.Now())
}

func (g *sessionGrants) removedWatchesLocked(next map[string]sessionGrant) []func() {
	callbacks := []func(){}
	for id, watches := range g.watches {
		if _, ok := next[id]; ok && !g.failed {
			continue
		}
		for _, watch := range watches {
			watch.timer.Stop()
			callbacks = append(callbacks, watch.close)
		}
		delete(g.watches, id)
	}
	return callbacks
}

func closeSessionWatches(callbacks []func()) {
	for _, close := range callbacks {
		go close()
	}
}

// replace grants a fresh ID and revokes the previous cookie in one durable
// update. This applies to editor upgrades, audience downgrades and re-logins.
func (g *sessionGrants) replace(previous, role string) (string, error) {
	if role != "editor" && role != "audience" {
		return "", fmt.Errorf("invalid presentation role")
	}
	g.mu.Lock()
	if g.failed {
		g.mu.Unlock()
		return "", fmt.Errorf("session grant persistence failed; repair the private session file before accepting logins")
	}
	next := map[string]sessionGrant{}
	for id, grant := range g.grants {
		if id != previous && grant.Expires.After(time.Now()) {
			next[id] = grant
		}
	}
	if len(next) >= sessionGrantLimit {
		g.mu.Unlock()
		return "", fmt.Errorf("room supports at most 256 active logins; sign out an existing session or wait for expiry")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		g.mu.Unlock()
		return "", err
	}
	id := hex.EncodeToString(random[:])
	next[id] = sessionGrant{Role: role, Expires: time.Now().UTC().Add(presentationSessionAge)}
	if err := g.persist(next); err != nil {
		g.mu.Unlock()
		return "", fmt.Errorf("persist session grant: %w", err)
	}
	g.grants = next
	callbacks := g.removedWatchesLocked(next)
	g.mu.Unlock()
	closeSessionWatches(callbacks)
	return id, nil
}

func (g *sessionGrants) revoke(id string) error {
	g.mu.Lock()
	next := map[string]sessionGrant{}
	for key, grant := range g.grants {
		if key != id && grant.Expires.After(time.Now()) {
			next[key] = grant
		}
	}
	err := g.persist(next)
	// A failed revocation never leaves live access running. Report the durable
	// failure, reject all room access, and close every connected author.
	if err != nil {
		g.failed = true
	}
	g.grants = next
	callbacks := g.removedWatchesLocked(next)
	g.mu.Unlock()
	closeSessionWatches(callbacks)
	if err != nil {
		return fmt.Errorf("session revocation could not be persisted; room access is disabled until restart with a new session secret: %w", err)
	}
	return nil
}

// watch binds an upgraded connection to the positive grant. Revocation closes
// it immediately; an independent timer closes it at the original login expiry.
func (g *sessionGrants) watch(id, key string, close func()) (func(), bool) {
	g.mu.Lock()
	grant, ok := g.grants[id]
	if g.failed || !ok || grant.Role != "editor" || !grant.Expires.After(time.Now()) {
		g.mu.Unlock()
		return func() {}, false
	}
	if g.watches[id] == nil {
		g.watches[id] = map[string]sessionGrantWatch{}
	}
	close = sync.OnceFunc(close)
	timer := time.AfterFunc(time.Until(grant.Expires), close)
	g.watches[id][key] = sessionGrantWatch{timer: timer, close: close}
	g.mu.Unlock()
	return func() {
		g.mu.Lock()
		defer g.mu.Unlock()
		if watch, ok := g.watches[id][key]; ok {
			watch.timer.Stop()
			delete(g.watches[id], key)
			if len(g.watches[id]) == 0 {
				delete(g.watches, id)
			}
		}
	}, true
}
