package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	slides "m31labs.dev/gosx-slides"
)

func historyFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"v1.sir":        "service api { sid: \"api\" }\n",
		"v2.sir":        "service renamed { sid: \"api\" }\njob worker\nrenamed -> worker\n",
		"v3.sir":        "service renamed { sid: \"api\" }\njob worker\ndatabase db\nrenamed -> worker\nworker -> db\n",
		"curation.yaml": "version: 1\nexplanations:\n  v2:\n    from: v1\n    beforeCaption: \"\"\n    afterCaption: Isolate bursts with a worker.\n",
		"history.yaml":  "version: 1\ntitle: Evolving service\ncuration: curation.yaml\nsnapshots:\n  - id: v1\n    label: PRIVATE revision\n    source: PRIVATE/source.go\n    path: v1.sir\n  - id: v2\n    path: v2.sir\n  - id: v3\n    path: v3.sir\n",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, filepath.Join(dir, "history.yaml")
}

func TestHistoryTourCLIPreservesCurationAndPublishesRunnableDeck(t *testing.T) {
	dir, manifest := historyFixture(t)
	out := filepath.Join(dir, "tour")
	if err := run([]string{"tour", "history", manifest, "--out", out}); err != nil {
		t.Fatal(err)
	}
	deck, err := slides.LoadIslandDeck(out)
	if err != nil || deck.Story == nil || len(deck.Slides) != 2 || len(deck.Story.Beats) != 4 {
		t.Fatalf("history deck: %v, %v", deck, err)
	}
	if deck.Story.Beats[0].Caption != "" || deck.Story.Beats[1].Caption != "Isolate bursts with a worker." {
		t.Fatal("lost explicit curation", deck.Story.Beats)
	}
	if err := run([]string{"story", "assert", out}); err != nil {
		t.Fatal(err)
	}
	markdown, err := os.ReadFile(filepath.Join(out, "deck.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(markdown), "PRIVATE") {
		t.Fatal("private provenance published")
	}
	report, err := os.ReadFile(filepath.Join(out, "tour.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tour slides.ArchitectureHistoryTour
	if err := json.Unmarshal(report, &tour); err != nil || tour.Snapshots[0].Source != "PRIVATE/source.go" {
		t.Fatal("private provenance not retained in report", err)
	}
	next := filepath.Join(dir, "regenerated")
	if err := run([]string{"tour", "history", manifest, "--curation", filepath.Join(out, "curation.json"), "--out", next}); err != nil {
		t.Fatal(err)
	}
	again, err := slides.LoadIslandDeck(next)
	if err != nil || again.Story.Beats[1].Caption != deck.Story.Beats[1].Caption {
		t.Fatal("regeneration changed explicit explanations", err)
	}
	if err := run([]string{"tour", "history", manifest, "--out", out}); err == nil {
		t.Fatal("overwrote existing tour")
	}
	after, err := os.ReadFile(filepath.Join(out, "deck.md"))
	if err != nil || string(after) != string(markdown) {
		t.Fatal("rejected generation altered prior source", err)
	}
}

func TestHistoryTourCLIRejectsEscapesUnknownFieldsAndMultipleDocuments(t *testing.T) {
	for _, kind := range []string{"escape", "unknown", "multiple", "version", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir, manifest := historyFixture(t)
			data, err := os.ReadFile(manifest)
			if err != nil {
				t.Fatal(err)
			}
			source := string(data)
			switch kind {
			case "escape":
				source = strings.Replace(source, "path: v1.sir", "path: ../outside.sir", 1)
			case "unknown":
				source += "secret: ignored\n"
			case "multiple":
				source += "---\nversion: 1\n"
			case "version":
				source = strings.Replace(source, "version: 1", "version: 2", 1)
			case "symlink":
				outside := filepath.Join(t.TempDir(), "private.sir")
				if err := os.WriteFile(outside, []byte("service private"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(filepath.Join(dir, "v1.sir")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, filepath.Join(dir, "v1.sir")); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			if err := os.WriteFile(manifest, []byte(source), 0644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(dir, "tour")
			if err := run([]string{"tour", "history", manifest, "--out", out}); err == nil {
				t.Fatal("invalid history accepted")
			}
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatal("rejected history created destination", err)
			}
		})
	}
}
