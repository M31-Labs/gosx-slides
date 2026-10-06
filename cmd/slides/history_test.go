package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	slides "m31labs.dev/gosx-slides"
)

func TestGitHistoryCLIRecordsActualRevisionPrivately(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("optional Git tool unavailable")
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		data, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture Git: %s %v", data, err)
		}
		return strings.TrimSpace(string(data))
	}
	git("init")
	var commits []string
	for _, source := range []string{"service api\n", "service api\njob worker\napi -> worker\n"} {
		if err := os.WriteFile(filepath.Join(dir, "model.sir"), []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
		git("add", "model.sir")
		git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "model")
		commits = append(commits, git("rev-parse", "HEAD"))
	}
	manifest := filepath.Join(t.TempDir(), "history.yaml")
	content := "version: 1\nsnapshots:\n  - id: initial\n    revision: HEAD~1\n    path: model.sir\n    source: PRIVATE authored metadata\n  - id: queued\n    revision: HEAD\n    path: model.sir\n"
	if err := os.WriteFile(manifest, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "tour")
	if err := run([]string{"tour", "history", manifest, "--repo", dir, "--out", out}); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(out, "tour.json"))
	var tour slides.ArchitectureHistoryTour
	if err != nil || json.Unmarshal(report, &tour) != nil || !strings.HasPrefix(tour.Snapshots[0].Source, commits[0]+":model.sir") || tour.Snapshots[1].Source != commits[1]+":model.sir" {
		t.Fatal("report lost resolved commit provenance", string(report), err)
	}
	markdown, err := os.ReadFile(filepath.Join(out, "deck.md"))
	if err != nil || strings.Contains(string(markdown), "PRIVATE") || strings.Contains(string(markdown), commits[0]) {
		t.Fatal("public deck contains private provenance", err)
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{"tour.json", "curation.json"} {
			info, err := os.Stat(filepath.Join(out, name))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("private report permissions", name, err)
			}
		}
	}
}

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
