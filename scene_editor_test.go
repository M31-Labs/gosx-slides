package slides

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sceneEditRef = `Src="actors.sir" Steps="steps.json" View="Actors"`
const sceneEditDiagram = `service api { label: "API" }
queue jobs { label: "Queue" }
api -> jobs: publishes "enqueue"
view "Actors" { include: [service "api", queue "jobs", edges: outgoing from "api"] layout { direction: left-right } }
`
const sceneEditSteps = `[{"label":"overview"},{"label":"accepted","durationMs":1200,"easing":"linear","patches":[{"target":"api","z":1,"scale":1.2}],"camera":{"z":9,"fov":40,"near":0.1,"far":100}}]`

func sceneEditingDeck(t *testing.T) *IslandDeck {
	t.Helper()
	return graphicsDeck(t, "# Scene\n\n<Scene3D "+sceneEditRef+" />\n", map[string]string{"actors.sir": sceneEditDiagram, "steps.json": sceneEditSteps})
}

func TestAuthoredSirenaSceneKeepsAttachmentsAndExplicitZero(t *testing.T) {
	deck := sceneEditingDeck(t)
	config, err := compileGraphic(deck.Dir, ComponentRef{Name: "Scene3D", Props: sceneEditRef})
	if err != nil {
		t.Fatal(err)
	}
	if config.MountAttrs["data-slide-scene-source"] != sceneGraphicID(ComponentRef{Name: "Scene3D", Props: sceneEditRef}) {
		t.Fatal("missing authored source identity")
	}
	var timeline graphicTimeline
	if err := json.Unmarshal([]byte(config.MountAttrs["data-slide-steps"].(string)), &timeline); err != nil {
		t.Fatal(err)
	}
	if len(timeline.Frames) != 2 || *timeline.Frames[1].DurationMS != 1200 {
		t.Fatal("authored cues were not compiled")
	}
	var beforeLabel, afterLabel, beforeEdge, afterEdge string
	for i, frame := range timeline.Frames {
		for _, command := range frame.Commands {
			raw, _ := json.Marshal(command.Data)
			if command.Kind == 0 && command.ObjectID == "label:api" {
				if i == 0 {
					beforeLabel = string(raw)
				} else {
					afterLabel = string(raw)
				}
			}
			if command.Kind == 0 && command.ObjectID == "edge:0" {
				if i == 0 {
					beforeEdge = string(raw)
				} else {
					afterEdge = string(raw)
				}
			}
		}
	}
	if beforeLabel == "" || beforeLabel == afterLabel || beforeEdge == "" || beforeEdge == afterEdge {
		t.Fatal("actor changes must retarget both labels and routes")
	}
	props := parseProps(sceneEditRef)
	payload, err := compileSirenaGraphic(deck.Dir, props, []byte(sceneEditDiagram), []byte(strings.Replace(sceneEditSteps, `"durationMs":1200`, `"durationMs":0`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	attrs, err := graphicStepAttrs(payload)
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal([]byte(attrs["data-slide-steps"].(string)), &timeline)
	if timeline.Frames[1].DurationMS == nil || *timeline.Frames[1].DurationMS != 0 {
		t.Fatal("explicit zero cue duration lost")
	}
	for _, invalid := range []string{strings.Replace(sceneEditSteps, `"target":"api"`, `"target":"missing"`, 1), strings.Replace(sceneEditSteps, `"scale":1.2`, `"scale":0`, 1), strings.Replace(sceneEditSteps, `"fov":40`, `"fov":180`, 1), strings.Replace(sceneEditSteps, `"label":"overview"`, `"unknown":true`, 1)} {
		if _, err := compileSirenaGraphic(deck.Dir, props, []byte(sceneEditDiagram), []byte(invalid)); err == nil {
			t.Fatal("invalid scene edit accepted", invalid)
		}
	}
}

func TestSceneEditorPreviewSaveSecurityAndConflicts(t *testing.T) {
	deck := sceneEditingDeck(t)
	path := filepath.Join(deck.Dir, "steps.json")
	os.Chmod(path, 0640)
	app, err := deck.NewServer(ServeOptions{Edit: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := app.Build()
	endpoint := "http://localhost/_slides/scene?graphic=" + sceneGraphicID(ComponentRef{Name: "Scene3D", Props: sceneEditRef})
	get := httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", endpoint, nil))
	if get.Code != 200 {
		t.Fatal(get.Code, get.Body.String())
	}
	var initial struct {
		Source, Revision, ContextRevision, Token string
		Timeline                                 graphicTimeline
	}
	json.Unmarshal(get.Body.Bytes(), &initial)
	edit := strings.Replace(sceneEditSteps, `"z":1,`, `"x":2,"z":3,`, 1)
	request := func(method, source, revision, context, token, origin, url string) (int, string) {
		raw, _ := json.Marshal(map[string]string{"source": source, "revision": revision, "contextRevision": context})
		req := httptest.NewRequest(method, url, bytes.NewReader(raw))
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Slides-Token", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Code, w.Body.String()
	}
	for _, row := range []struct {
		method, token, origin, url string
		want                       int
	}{
		{"POST", "", "http://localhost", endpoint, 403}, {"PUT", initial.Token, "", endpoint, 403}, {"PUT", initial.Token, "http://evil.example", endpoint, 403}, {"PUT", initial.Token, "http://localhost", "http://localhost/_slides/scene?graphic=../../steps.json", 400},
	} {
		code, body := request(row.method, edit, initial.Revision, initial.ContextRevision, row.token, row.origin, row.url)
		if code != row.want {
			t.Fatal(code, body)
		}
	}
	code, body := request("POST", edit, initial.Revision, initial.ContextRevision, initial.Token, "http://localhost", endpoint)
	if code != 200 {
		t.Fatal(code, body)
	}
	unchanged, _ := os.ReadFile(path)
	if string(unchanged) != sceneEditSteps {
		t.Fatal("preview wrote source")
	}
	code, body = request("PUT", edit, initial.Revision, initial.ContextRevision, initial.Token, "http://localhost", endpoint)
	if code != 200 {
		t.Fatal(code, body)
	}
	saved, _ := os.ReadFile(path)
	if string(saved) != edit {
		t.Fatal("cue save was not persisted")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0640 {
		t.Fatal("cue file permissions changed")
	}
	recovery, _ := filepath.Glob(filepath.Join(deck.Dir, ".slides-history-*", "steps.json"))
	if len(recovery) != 1 {
		t.Fatal("cue recovery missing")
	}
	previous, _ := os.ReadFile(recovery[0])
	if string(previous) != sceneEditSteps {
		t.Fatal("cue recovery differs")
	}
	code, _ = request("PUT", sceneEditSteps, initial.Revision, initial.ContextRevision, initial.Token, "http://localhost", endpoint)
	if code != 409 {
		t.Fatal("stale cue save accepted", code)
	}
	get = httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", endpoint, nil))
	json.Unmarshal(get.Body.Bytes(), &initial)
	os.WriteFile(filepath.Join(deck.Dir, "actors.sir"), []byte(sceneEditDiagram+"\n// External edit\n"), 0644)
	code, _ = request("PUT", sceneEditSteps, initial.Revision, initial.ContextRevision, initial.Token, "http://localhost", endpoint)
	if code != 409 {
		t.Fatal("changed scene context accepted", code)
	}
	if err := os.Rename(path, path+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path+".original", path); err != nil {
		t.Fatal(err)
	}
	get = httptest.NewRecorder()
	handler.ServeHTTP(get, httptest.NewRequest("GET", endpoint, nil))
	if get.Code != 400 {
		t.Fatal("authored symlink accepted", get.Code)
	}
	for _, opts := range []ServeOptions{{}, {Edit: true, Static: true}} {
		app, err := deck.NewServer(opts)
		if err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		app.Build().ServeHTTP(w, httptest.NewRequest(http.MethodGet, endpoint, nil))
		if w.Code == 200 {
			t.Fatal("scene editing exposed on a read-only server")
		}
	}
}
