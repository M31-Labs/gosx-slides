package slides

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSessionRolesCSRFAndSourceSaves(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, DeckFileName), []byte("# Shared\n\n<!-- Private presenter wording -->\n"), 0644); err != nil {
		t.Fatal(err)
	}
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	options := SessionOptions{EditorToken: strings.Repeat("e", 40), AudienceToken: strings.Repeat("a", 40)}
	app, err := deck.NewServer(ServeOptions{Edit: true, Sessions: &options})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(app.Build())
	defer server.Close()
	client := server.Client()
	client.Jar, _ = cookiejar.New(nil)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	request := func(method, path, body, csrf, sourceToken, origin string) (int, string, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if method != http.MethodGet {
			req.Header.Set("Content-Type", "application/json")
		}
		if path == "/_slides/session" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		req.Header.Set("X-CSRF-Token", csrf)
		req.Header.Set("X-Slides-Token", sourceToken)
		req.Header.Set("Origin", origin)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, string(data), res.Header
	}
	if status, _, _ := request("GET", "/", "", "", "", ""); status != 303 {
		t.Fatal("anonymous page did not request a session", status)
	}
	for _, path := range []string{"/_slides/source", "/gosx/islands/Counter.json", "/presenter/events"} {
		if status, _, _ := request("GET", path, "", "", "", ""); status != 401 {
			t.Fatal("anonymous private route exposed", path, status)
		}
	}
	form := func(token string) string { return url.Values{"token": {token}}.Encode() }
	for _, origin := range []string{"", "https://foreign.example"} {
		if status, _, _ := request("POST", "/_slides/session", form(options.EditorToken), "", "", origin); status != 403 {
			t.Fatal("forged login accepted", status)
		}
	}
	if status, _, _ := request("POST", "/_slides/session", form("bad"), "", "", server.URL); status != 401 {
		t.Fatal("invalid token accepted", status)
	}
	status, _, headers := request("POST", "/_slides/session", form(options.AudienceToken), "", "", server.URL)
	if status != 303 {
		t.Fatal("audience login failed", status)
	}
	cookies := (&http.Response{Header: headers}).Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].MaxAge != 8*60*60 || strings.Contains(cookies[0].Value, "audience") {
		t.Fatal("session cookie contract not applied")
	}
	csrfPattern := regexp.MustCompile(`name="slides-csrf" content="([^"]+)"`)
	status, audienceHTML, headers := request("GET", "/", "", "", "", "")
	if status != 200 || strings.Contains(audienceHTML, "Private presenter wording") || strings.Contains(audienceHTML, `<meta name="slides-edit"`) || !strings.Contains(audienceHTML, `data-session-role="audience"`) || !strings.Contains(headers.Get("Cache-Control"), "private") {
		t.Fatalf("audience contract: status=%d notes=%t editor=%t role=%t cache=%q", status, strings.Contains(audienceHTML, "Private presenter wording"), strings.Contains(audienceHTML, `<meta name="slides-edit"`), strings.Contains(audienceHTML, `data-session-role="audience"`), headers.Get("Cache-Control"))
	}
	match := csrfPattern.FindStringSubmatch(audienceHTML)
	if len(match) != 2 {
		t.Fatal("audience session CSRF token missing")
	}
	csrf := match[1]
	for _, path := range []string{"/_slides/source", "/_slides/scene", "/remote", "/?present", "/_slides/team"} {
		if status, _, _ := request("GET", path, "", "", "", ""); status != 403 {
			t.Fatal("audience reached editor route", path, status)
		}
	}
	if status, _, _ := request("POST", "/presenter/state", `{"index":1}`, csrf, "", server.URL); status != 403 {
		t.Fatal("audience advanced presenter", status)
	}
	if status, _, _ := request("POST", "/_slides/session", form(options.EditorToken), csrf, "", server.URL); status != 303 {
		t.Fatal("editor login failed", status)
	}
	status, editorHTML, _ := request("GET", "/", "", "", "", "")
	if status != 200 || !strings.Contains(editorHTML, "Private presenter wording") || !strings.Contains(editorHTML, `<meta name="slides-edit"`) {
		t.Fatalf("editor contract: status=%d notes=%t editor=%t role=%t", status, strings.Contains(editorHTML, "Private presenter wording"), strings.Contains(editorHTML, `<meta name="slides-edit"`), strings.Contains(editorHTML, `data-session-role="editor"`))
	}
	csrf = csrfPattern.FindStringSubmatch(editorHTML)[1]
	if status, _, _ := request("POST", "/presenter/state", `{"index":1}`, "", "", server.URL); status != 403 {
		t.Fatal("session presenter write allowed without CSRF", status)
	}
	if status, _, _ := request("POST", "/presenter/state", `{"index":1}`, csrf, "", "https://foreign.example"); status != 403 {
		t.Fatal("session presenter write allowed from foreign origin", status)
	}
	if status, _, _ := request("POST", "/presenter/state", `{"index":1}`, csrf, "", server.URL); status != 204 {
		t.Fatal("editor presenter write rejected", status)
	}
	status, source, _ := request("GET", "/_slides/source", "", "", "", "")
	var revision struct{ Revision, Token string }
	if status != 200 || json.Unmarshal([]byte(source), &revision) != nil {
		t.Fatal("editor source unavailable", status)
	}
	body, _ := json.Marshal(map[string]string{"source": "# Authenticated edit\n", "revision": revision.Revision})
	if status, _, _ := request("PUT", "/_slides/source", string(body), csrf, revision.Token, server.URL); status != 200 {
		t.Fatal("authenticated remote editor save failed", status)
	}
	if status, _, _ := request("POST", "/_slides/logout", "{}", "", "", server.URL); status != 403 {
		t.Fatal("logout accepted without CSRF", status)
	}
	if status, _, _ := request("POST", "/_slides/logout", "{}", csrf, "", server.URL); status != 303 {
		t.Fatal("logout failed", status)
	}
	if status, _, _ := request("GET", "/_slides/source", "", "", "", ""); status != 401 {
		t.Fatal("logged out browser retained source access", status)
	}
}

func TestServeAccessRejectsUnsafeConfigurations(t *testing.T) {
	if err := validateServeAccess(ServeOptions{Addr: "0.0.0.0:8080"}); err == nil {
		t.Fatal("public listener accepted without sessions")
	}
	for _, addr := range []string{"127.0.0.1:8080", "[::1]:8080", "localhost:8080"} {
		if err := validateServeAccess(ServeOptions{Addr: addr}); err != nil {
			t.Fatal(err)
		}
	}
	deck, err := LoadIslandDeck("examples/real-deck")
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range []ServeOptions{
		{Static: true, Sessions: &SessionOptions{EditorToken: strings.Repeat("e", 32)}},
		{Sessions: &SessionOptions{EditorToken: "short"}},
		{Sessions: &SessionOptions{EditorToken: strings.Repeat("e", 32), AudienceToken: "short"}},
		{Sessions: &SessionOptions{EditorToken: strings.Repeat("e", 32), AudienceToken: strings.Repeat("e", 32)}},
		{Sessions: &SessionOptions{EditorToken: strings.Repeat("e", 32), Secret: "short"}},
	} {
		if _, err := deck.NewServer(options); err == nil {
			t.Fatal("invalid session configuration accepted")
		}
	}
}

func TestSessionRejectsTamperedCookieAndOversizedLogin(t *testing.T) {
	deck, err := LoadIslandDeck("examples/real-deck")
	if err != nil {
		t.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{Sessions: &SessionOptions{EditorToken: strings.Repeat("e", 32), AllowInsecure: true}})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Build()
	req := httptest.NewRequest("GET", "http://localhost/_slides/source", nil)
	req.AddCookie(&http.Cookie{Name: "slides_session", Value: "forged.editor"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("forged cookie accepted", w.Code)
	}
	req = httptest.NewRequest("POST", "http://localhost/_slides/session", bytes.NewBufferString("token="+strings.Repeat("e", 9000)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "http://localhost")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatal("unbounded login form accepted", w.Code)
	}
}
