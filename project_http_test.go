package slides

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestProjectEndpointOriginRoleAndConflict(t *testing.T) {
	p, dir := authorProjectFixture(t)
	deck, err := LoadIslandDeck(dir)
	if err != nil {
		t.Fatal(err)
	}
	app, err := deck.NewServer(ServeOptions{Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Build()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "http://localhost/_slides/project", nil))
	if response.Code != 200 {
		t.Fatalf("index %d %s", response.Code, response.Body.String())
	}
	var index struct {
		Token string `json:"token"`
		ProjectIndex
	}
	if err := json.Unmarshal(response.Body.Bytes(), &index); err != nil {
		t.Fatal(err)
	}
	doc, _ := p.Read("sections/body.md")
	edit := ProjectEdit{doc.Path, doc.Source + "\nEdited\n", doc.Revision, doc.ContextRevision}
	encoded, _ := json.Marshal(edit)
	for _, test := range []struct {
		host, origin, token, role string
		status                    int
	}{
		{"localhost", "http://evil.example", index.Token, "", 403},
		{"localhost", "http://localhost", "", "", 403},
		{"custom.example", "http://custom.example", index.Token, "", 403},
		{"localhost", "http://localhost", index.Token, "audience", 403},
		{"localhost", "http://localhost", index.Token, "editor", 200},
		{"localhost", "http://localhost", index.Token, "editor", 409},
	} {
		req := httptest.NewRequest("PUT", "http://"+test.host+"/_slides/project", bytes.NewReader(encoded))
		req.Header.Set("Origin", test.origin)
		req.Header.Set("X-Slides-Token", test.token)
		if test.role != "" {
			req = req.WithContext(context.WithValue(req.Context(), slidesSessionContextKey{}, slidesSessionAccess{role: test.role}))
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if out.Code != test.status {
			t.Fatalf("%+v: %d %s", test, out.Code, out.Body.String())
		}
	}
	for _, role := range []string{"audience", ""} {
		req := httptest.NewRequest("GET", "http://evil.example/_slides/project?file=deck.md", nil)
		if role != "" {
			req = req.WithContext(context.WithValue(req.Context(), slidesSessionContextKey{}, slidesSessionAccess{role: role}))
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if out.Code != 403 {
			t.Fatalf("source disclosure: %d", out.Code)
		}
	}
}
