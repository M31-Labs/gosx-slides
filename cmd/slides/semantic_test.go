package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	slides "m31labs.dev/gosx-slides"
)

func TestTourCommandCreatesRunnableStoryWithoutOverwriting(t *testing.T) {
	parent := t.TempDir()
	before, after := filepath.Join(parent, "before.sir"), filepath.Join(parent, "after.sir")
	for path, source := range map[string]string{
		before: `service api { sid: "api", label: "API" }`,
		after:  `service renamed { sid: "api", label: "Version two" }`,
	} {
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(parent, "tour")
	if err := run([]string{"tour", before, after, "--out", out}); err != nil {
		t.Fatal(err)
	}
	deck, err := slides.LoadIslandDeck(out)
	if err != nil || deck.Story == nil || len(deck.Story.Beats) != 2 {
		t.Fatal("generated tour must compile as an addressed story", err)
	}
	if err := run([]string{"story", "assert", out}); err != nil {
		t.Fatal("generated tour failed assertions", err)
	}
	source, err := os.ReadFile(filepath.Join(out, "deck.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"tour", after, before, "--out", out}); err == nil {
		t.Fatal("existing tour directory was overwritten")
	}
	next, err := os.ReadFile(filepath.Join(out, "deck.md"))
	if err != nil || string(next) != string(source) {
		t.Fatal("rejected tour changed existing source", err)
	}
}

func TestAudienceEditingCombinationRejectsBeforeServing(t *testing.T) {
	for _, flag := range []string{"--edit", "--collab", "--watch"} {
		if err := run([]string{"serve", "missing-deck", "--audience", "engineers", flag}); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
			t.Fatal("filtered authoring must fail before loading or listening", flag, err)
		}
	}
}
