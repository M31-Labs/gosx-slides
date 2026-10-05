package slides

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSessionGrantRotationRestartAndRevocation(t *testing.T) {
	dir := t.TempDir()
	grants, err := newSessionGrants(dir, "scope")
	if err != nil {
		t.Fatal(err)
	}
	editor, err := grants.replace("", "editor")
	if err != nil || !grants.valid(editor, "editor") {
		t.Fatal("editor grant", err)
	}
	closed := make(chan struct{}, 1)
	cancel, ok := grants.watch(editor, "socket", func() { closed <- struct{}{} })
	if !ok {
		t.Fatal("editor socket not watched")
	}
	defer cancel()
	info, err := os.Stat(filepath.Join(dir, sessionGrantFile))
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatal("session file is not private", err)
	}
	restarted, err := newSessionGrants(dir, "scope")
	if err != nil || !restarted.valid(editor, "editor") {
		t.Fatal("stable restart lost login", err)
	}
	rotated, err := newSessionGrants(dir, "changed-scope")
	if err != nil || rotated.valid(editor, "editor") {
		t.Fatal("changed keys retained login", err)
	}
	audience, err := grants.replace(editor, "audience")
	if err != nil || grants.valid(editor, "editor") || !grants.valid(audience, "audience") {
		t.Fatal("downgrade retained editor access", err)
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("downgrade did not close socket")
	}
	if _, ok := grants.watch(editor, "old", func() {}); ok {
		t.Fatal("old socket watched")
	}
	if _, ok := grants.watch(audience, "viewer", func() {}); ok {
		t.Fatal("audience socket watched")
	}
	if err := grants.revoke(audience); err != nil {
		t.Fatal(err)
	}
	restarted, err = newSessionGrants(dir, "scope")
	if err != nil || restarted.valid(editor, "editor") || restarted.valid(audience, "audience") {
		t.Fatal("revoked cookie replay survived restart", err)
	}
}

func TestSessionGrantExpiryLimitAndPersistenceFailure(t *testing.T) {
	dir := t.TempDir()
	grants, err := newSessionGrants(dir, "scope")
	if err != nil {
		t.Fatal(err)
	}
	editor, err := grants.replace("", "editor")
	if err != nil {
		t.Fatal(err)
	}
	grants.mu.Lock()
	grant := grants.grants[editor]
	grant.Expires = time.Now().Add(30 * time.Millisecond)
	grants.grants[editor] = grant
	grants.mu.Unlock()
	closed := make(chan struct{}, 1)
	cancel, ok := grants.watch(editor, "expiring", func() { closed <- struct{}{} })
	if !ok {
		t.Fatal("expiring socket not watched")
	}
	defer cancel()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("expiry did not close socket")
	}
	if grants.valid(editor, "editor") {
		t.Fatal("expired grant authorized event or draft delivery")
	}
	grants.mu.Lock()
	for index := 0; index < sessionGrantLimit; index++ {
		grants.grants[fmt.Sprintf("%064x", index)] = sessionGrant{Role: "editor", Expires: time.Now().Add(time.Hour)}
	}
	grants.mu.Unlock()
	if _, err := grants.replace("", "editor"); err == nil {
		t.Fatal("unbounded session logins")
	}
	// Replacing one active login does not consume an additional grant slot.
	editor, err = grants.replace(fmt.Sprintf("%064x", 1), "editor")
	if err != nil {
		t.Fatal(err)
	}
	failureClosed := make(chan struct{}, 1)
	cancel, ok = grants.watch(editor, "failure", func() { failureClosed <- struct{}{} })
	if !ok {
		t.Fatal("active socket not watched")
	}
	defer cancel()
	if err := os.Remove(filepath.Join(dir, sessionGrantFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, sessionGrantFile), 0700); err != nil {
		t.Fatal(err)
	}
	if err := grants.revoke(editor); err == nil {
		t.Fatal("unpersisted revocation reported success")
	}
	select {
	case <-failureClosed:
	case <-time.After(time.Second):
		t.Fatal("failed persistence left socket open")
	}
	if grants.valid(editor, "editor") {
		t.Fatal("failed persistence retained access")
	}
	if _, err := grants.replace("", "editor"); err == nil {
		t.Fatal("failed persistence accepted login")
	}
	if _, err := newSessionGrants(dir, "scope"); err == nil {
		t.Fatal("restart accepted invalid private ledger")
	}
	if paths, _ := filepath.Glob(filepath.Join(dir, ".slides-session-*.tmp")); len(paths) != 0 {
		t.Fatal("failed save left private temporary files", paths)
	}
}

func TestSessionGrantRejectsUnsafePrivateState(t *testing.T) {
	for _, data := range []string{
		`{"version":2,"scope":"scope","grants":{}}`,
		`{"version":1,"scope":"scope","grants":{},"unknown":true}`,
		`{"version":1,"scope":"scope","grants":{}} {}`,
		`{"version":1,"scope":"scope","grants":{"bad":{"role":"editor","expires":"2099-01-01T00:00:00Z"}}}`,
		strings.Repeat("x", sessionGrantFileLimit+1),
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, sessionGrantFile), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := newSessionGrants(dir, "scope"); err == nil {
			t.Fatal("invalid state accepted")
		}
	}
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.json")
	if err := os.WriteFile(outside, []byte(`{"version":1,"scope":"scope","grants":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, sessionGrantFile)); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
	} else if _, err := newSessionGrants(dir, "scope"); err == nil {
		t.Fatal("symlinked state accepted")
	}
}

func TestSessionCookiesCannotReplayAfterRoleChangeOrLogout(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte("# Private room\n\n<!-- Speaker notes -->\n"), 0600); err != nil {
		t.Fatal(err)
	}
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	options := SessionOptions{EditorToken: strings.Repeat("e", 40), AudienceToken: strings.Repeat("a", 40), Secret: strings.Repeat("s", 40), AllowInsecure: true}
	newServer := func() *httptest.Server {
		t.Helper()
		app, err := deck.NewServer(ServeOptions{Edit: true, Sessions: &options})
		if err != nil {
			t.Fatal(err)
		}
		return httptest.NewServer(app.Build())
	}
	server := newServer()
	defer func() { server.Close() }()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request := func(method, path, body string, cookie *http.Cookie, csrf string) (int, string, *http.Cookie) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", server.URL)
		req.Header.Set("X-CSRF-Token", csrf)
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		updated := cookie
		for _, candidate := range response.Cookies() {
			if candidate.Name == "slides_session" {
				updated = candidate
			}
		}
		return response.StatusCode, string(data), updated
	}
	status, _, editorCookie := request("POST", "/_slides/session", url.Values{"token": {options.EditorToken}}.Encode(), nil, "")
	if status != 303 || editorCookie == nil {
		t.Fatal("login failed", status)
	}
	status, page, editorCookie := request("GET", "/", "", editorCookie, "")
	if status != 200 {
		t.Fatal("editor unavailable", status)
	}
	csrfMatch := regexp.MustCompile(`name="slides-csrf" content="([^"]+)"`).FindStringSubmatch(page)
	if len(csrfMatch) != 2 {
		t.Fatal("CSRF token missing")
	}
	csrf := csrfMatch[1]
	for _, path := range []string{"/" + sessionGrantFile, "/public/" + sessionGrantFile} {
		if status, _, _ := request("GET", path, "", editorCookie, ""); status != 404 {
			t.Fatal("private session state exposed", path, status)
		}
	}
	server.Close()
	server = newServer()
	if status, _, _ := request("GET", "/_slides/source", "", editorCookie, ""); status != 200 {
		t.Fatal("stable-secret restart did not preserve active login", status)
	}
	status, _, audienceCookie := request("POST", "/_slides/session", url.Values{"token": {options.AudienceToken}}.Encode(), editorCookie, csrf)
	if status != 303 {
		t.Fatal("role downgrade failed", status)
	}
	if status, _, _ := request("GET", "/_slides/source", "", editorCookie, ""); status != 401 {
		t.Fatal("old editor cookie replayed after downgrade", status)
	}
	if status, _, _ := request("GET", "/", "", audienceCookie, ""); status != 200 {
		t.Fatal("new audience role unavailable", status)
	}
	if status, _, _ := request("GET", "/_slides/source", "", audienceCookie, ""); status != 403 {
		t.Fatal("new audience retained source access", status)
	}
	status, page, audienceCookie = request("GET", "/", "", audienceCookie, "")
	csrf = regexp.MustCompile(`name="slides-csrf" content="([^"]+)"`).FindStringSubmatch(page)[1]
	if status, _, _ := request("POST", "/_slides/logout", "", audienceCookie, csrf); status != 303 {
		t.Fatal("logout failed", status)
	}
	server.Close()
	server = newServer()
	for _, cookie := range []*http.Cookie{editorCookie, audienceCookie} {
		if status, _, _ := request("GET", "/_slides/source", "", cookie, ""); status != 401 {
			t.Fatal("revoked cookie replayed after restart", status)
		}
	}
	var state sessionGrantState
	data, err := os.ReadFile(filepath.Join(dir, sessionGrantFile))
	if err != nil || json.Unmarshal(data, &state) != nil || len(state.Grants) != 0 {
		t.Fatal("logout left active durable grant", err)
	}
	out := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "build"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := exportSPA(dir, deck, "<!doctype html><html><head></head><body>Public presentation</body></html>", out, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Fatal(err)
	}
	if err := filepath.WalkDir(out, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".slides-session") {
			t.Errorf("export included private session state: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
