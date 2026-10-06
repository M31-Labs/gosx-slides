package slides

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"

	"m31labs.dev/gosx/server"
)

func mountProjectEditor(app *server.App, deck *IslandDeck, token string, mu *sync.Mutex) error {
	project, err := NewAuthorProject(deck.Dir)
	if err != nil {
		return err
	}
	project.mu = mu
	app.Mount("/_slides/project", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Type", "application/json")
		fail := func(err error) {
			status := http.StatusBadRequest
			var projectErr *ProjectError
			if errors.As(err, &projectErr) {
				status = projectErr.Status
			}
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		}
		if r.Method != "GET" && r.Method != "POST" && r.Method != "PUT" {
			w.Header().Set("Allow", "GET, POST, PUT")
			fail(projectFailure(405, "method not allowed"))
			return
		}
		if err := authorizeSourceRequest(r, token, r.Method != "GET"); err != nil {
			fail(projectFailure(403, err.Error()))
			return
		}
		if r.Method == "GET" {
			if name := r.URL.Query().Get("file"); name != "" {
				doc, err := project.Read(name)
				if err != nil {
					fail(err)
					return
				}
				json.NewEncoder(w).Encode(doc)
			} else {
				index, err := project.List()
				if err != nil {
					fail(err)
					return
				}
				json.NewEncoder(w).Encode(struct {
					ProjectIndex
					Token string `json:"token"`
				}{index, token})
			}
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 6*maxSourceBytes+16384)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if r.Method == "POST" {
			var input struct {
				File   string `json:"file"`
				Source string `json:"source"`
			}
			if err := decoder.Decode(&input); err != nil {
				fail(projectFailure(400, "invalid project diagnosis"))
				return
			}
			if decoder.Decode(new(any)) != io.EOF {
				fail(projectFailure(400, "expected one project diagnosis"))
				return
			}
			report, err := project.Diagnose(input.File, input.Source)
			if err != nil {
				fail(err)
				return
			}
			json.NewEncoder(w).Encode(report)
			return
		}
		var edit ProjectEdit
		if err := decoder.Decode(&edit); err != nil {
			fail(projectFailure(400, "invalid project edit"))
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			fail(projectFailure(400, "expected one project edit"))
			return
		}
		saved, err := project.Write(edit)
		if err != nil {
			fail(err)
			return
		}
		json.NewEncoder(w).Encode(saved)
	}))
	return nil
}
