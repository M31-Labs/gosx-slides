package main

import (
	"os"
	"reflect"
	"testing"
)

func TestTakeStringFlag(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		fallback  string
		wantValue string
		wantRest  []string
		wantErr   bool
	}{
		{"--name value", []string{"--theme", "neon", "deck"}, "aurora", "neon", []string{"deck"}, false},
		{"--name=value", []string{"--theme=swiss", "deck"}, "aurora", "swiss", []string{"deck"}, false},
		{"-name value", []string{"-theme", "paper"}, "aurora", "paper", nil, false},
		{"default when absent", []string{"deck"}, "aurora", "aurora", []string{"deck"}, false},
		{"missing value errors", []string{"--theme"}, "aurora", "", nil, true},
		{"unknown flag passes through", []string{"--wat", "deck"}, "aurora", "aurora", []string{"--wat", "deck"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, rest, err := takeStringFlag(tc.args, "theme", tc.fallback)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got != tc.wantValue {
				t.Errorf("value = %q, want %q", got, tc.wantValue)
			}
			if !reflect.DeepEqual(rest, tc.wantRest) {
				t.Errorf("rest = %v, want %v", rest, tc.wantRest)
			}
		})
	}
}

func TestTakeBoolFlag(t *testing.T) {
	got, rest := takeBoolFlag([]string{"--strict", "deck"}, "strict")
	if !got || !reflect.DeepEqual(rest, []string{"deck"}) {
		t.Errorf("present: got=%v rest=%v", got, rest)
	}
	got, rest = takeBoolFlag([]string{"deck"}, "strict")
	if got || !reflect.DeepEqual(rest, []string{"deck"}) {
		t.Errorf("absent: got=%v rest=%v", got, rest)
	}
}

func TestTakeIntFlag(t *testing.T) {
	got, _, err := takeIntFlag([]string{"--port", "9000"}, "port", 8080)
	if err != nil || got != 9000 {
		t.Fatalf("got=%d err=%v, want 9000", got, err)
	}
	if got, _, _ := takeIntFlag([]string{"deck"}, "port", 8080); got != 8080 {
		t.Errorf("default = %d, want 8080", got)
	}
	if _, _, err := takeIntFlag([]string{"--port", "abc"}, "port", 8080); err == nil {
		t.Error("non-integer port should error")
	}
}

func TestDeckDir(t *testing.T) {
	if d := deckDir(nil); d != "." {
		t.Errorf("no arg -> %q, want .", d)
	}
	if d := deckDir([]string{"some/path/deck.md"}); d != "some/path" {
		t.Errorf("deck.md path -> %q, want parent dir", d)
	}
	if d := deckDir([]string{"my-deck"}); d != "my-deck" {
		t.Errorf("dir -> %q, want my-deck", d)
	}
}

// TestRunDispatch smoke-tests the command router on the real example deck (the
// lane-routing + default-mismatch logic the audit flagged as 0%-covered).
func TestRunDispatch(t *testing.T) {
	silence := func() func() {
		old := os.Stdout
		devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		os.Stdout = devnull
		return func() { os.Stdout = old; devnull.Close() }
	}
	restore := silence()
	defer restore()

	deck := "../../examples/canopy-migration"
	ok := [][]string{
		{"version"},
		{"help"},
		{"themes"},
		{"check", deck},
		{"validate", deck, "--strict"},
		{"inspect", deck, "--json"},
		{"components", deck},
	}
	for _, args := range ok {
		if err := run(args); err != nil {
			t.Errorf("run(%v) = %v, want nil", args, err)
		}
	}
	if err := run([]string{"definitely-not-a-command"}); err == nil {
		t.Error("unknown command should error")
	}
}
