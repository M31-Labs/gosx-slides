package slides

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAudienceSPARejectsStaleExcludedProgramsBeforeWriting(t *testing.T) {
	deck := exportAssetTestDeck(t)
	writeCompositionFiles(t, deck.Dir, map[string]string{
		"deck.md":     audienceTestSource,
		"Counter.gsx": "package main\n\n//gosx:island\nfunc Counter(props any) Node {\n return <span>ENGINEER PROGRAM {props.Initial}</span>\n}\n",
	})
	engineers, err := LoadIslandDeckAudience(deck.Dir, "engineers")
	if err != nil {
		t.Fatal(err)
	}
	leaders, err := LoadIslandDeckAudience(deck.Dir, "leaders")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := stageDeckIslandPrograms(engineers); err != nil {
		t.Fatal(err)
	}
	if err := exportSPA(deck.Dir, engineers, "ENGINEER INDEX", out, false); err != nil {
		t.Fatal(err)
	}
	programPath := filepath.Join(out, "gosx/islands/Counter.json")
	program, err := os.ReadFile(programPath)
	if err != nil || len(program) == 0 {
		t.Fatal("engineer program not exported", err)
	}
	// Re-exporting the same audience remains supported.
	if err := exportSPA(deck.Dir, engineers, "ENGINEER INDEX", out, false); err != nil {
		t.Fatal(err)
	}
	if err := stageDeckIslandPrograms(leaders); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(deck.Dir, "build/islands/Counter.json")); !os.IsNotExist(err) {
		t.Fatal("excluded program left in staging", err)
	}
	if err := exportSPA(deck.Dir, leaders, "LEADER INDEX", out, false); err == nil || !strings.Contains(err.Error(), "unplanned island program") {
		t.Fatal("reused output retained an excluded program", err)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "index.html")); string(got) != "ENGINEER INDEX" {
		t.Fatal("failed export changed previous index")
	}
	if got, _ := os.ReadFile(programPath); string(got) != string(program) {
		t.Fatal("failed export changed previous program")
	}
	fresh := t.TempDir()
	if err := exportSPA(deck.Dir, leaders, "LEADER INDEX", fresh, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fresh, "gosx/islands/Counter.json")); !os.IsNotExist(err) {
		t.Fatal("fresh leader export published excluded program", err)
	}
}

func TestSPARejectsUnplannedManagedIslandArtifacts(t *testing.T) {
	for _, name := range []string{"islands/Unknown.json", "islands/hash.gxi", "islands/nested/hash.json", "assets/islands/hash.json"} {
		for _, location := range []string{"source", "output"} {
			t.Run(location+"/"+name, func(t *testing.T) {
				deck := exportAssetTestDeck(t)
				out := t.TempDir()
				writeCompositionFiles(t, out, map[string]string{"index.html": "PREVIOUS"})
				if location == "source" {
					writeCompositionFiles(t, deck.Dir, map[string]string{"build/" + name: "UNSELECTED PROGRAM"})
				} else {
					writeCompositionFiles(t, out, map[string]string{"gosx/" + name: "UNSELECTED PROGRAM"})
				}
				if err := exportSPA(deck.Dir, deck, "replacement", out, false); err == nil || !strings.Contains(err.Error(), "unplanned island program") {
					t.Fatal("unknown managed artifact accepted", err)
				}
				if got, _ := os.ReadFile(filepath.Join(out, "index.html")); string(got) != "PREVIOUS" {
					t.Fatal("preflight changed previous HTML")
				}
				if _, err := os.Stat(filepath.Join(out, "gosx/bootstrap.js")); !os.IsNotExist(err) {
					t.Fatal("assets copied before rejecting unplanned program")
				}
			})
		}
	}
}

func TestStageIslandProgramsRejectsUnsupportedArtifactsBeforeCleanup(t *testing.T) {
	for _, name := range []string{"hash.gxi", "nested/program.json"} {
		t.Run(name, func(t *testing.T) {
			deck := exportAssetTestDeck(t)
			writeCompositionFiles(t, deck.Dir, map[string]string{"build/islands/Old.json": "PREVIOUS", "build/islands/" + name: "UNKNOWN"})
			if err := StageIslandPrograms(deck.Dir); err == nil || !strings.Contains(err.Error(), "unsupported artifact") {
				t.Fatal("unsupported staged program accepted", err)
			}
			if got, _ := os.ReadFile(filepath.Join(deck.Dir, "build/islands/Old.json")); string(got) != "PREVIOUS" {
				t.Fatal("unsupported staging layout deleted previous programs")
			}
		})
	}
}

func TestStageIslandProgramsRejectsSymlinkedBuildDirectory(t *testing.T) {
	deck := exportAssetTestDeck(t)
	out := t.TempDir()
	writeCompositionFiles(t, out, map[string]string{"Old.json": "AUTHORED"})
	if err := os.Symlink(out, filepath.Join(deck.Dir, "build/islands")); err != nil {
		t.Skip("symlink creation unavailable", err)
	}
	if err := StageIslandPrograms(deck.Dir); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatal("staged programs through a symlink", err)
	}
	if got, _ := os.ReadFile(filepath.Join(out, "Old.json")); string(got) != "AUTHORED" {
		t.Fatal("outside authored program deleted")
	}
}

func TestStageIslandProgramsPropagatesReadAndRemovalFailures(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires ordinary Unix directory permissions")
	}
	for _, test := range []struct {
		name    string
		mode    os.FileMode
		message string
	}{{"read", 0000, "read island build dir"}, {"remove", 0555, "remove stale island program"}} {
		t.Run(test.name, func(t *testing.T) {
			deck := exportAssetTestDeck(t)
			path := filepath.Join(deck.Dir, "build/islands")
			writeCompositionFiles(t, deck.Dir, map[string]string{"build/islands/Old.json": "PREVIOUS"})
			if err := os.Chmod(path, test.mode); err != nil {
				t.Fatal(err)
			}
			defer os.Chmod(path, 0755)
			if err := StageIslandPrograms(deck.Dir); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatal("staging ignored file operation failure", err)
			}
			if err := os.Chmod(path, 0755); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(filepath.Join(path, "Old.json")); string(got) != "PREVIOUS" {
				t.Fatal("failed staging modified previous program")
			}
		})
	}
}
