package slides

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"m31labs.dev/gosx"
	"m31labs.dev/gosx/island"
	"m31labs.dev/gosx/sim"
)

func TestSimulationReplaySeekBranchAndOwnership(t *testing.T) {
	config := SimulationConfig{Seed: 1729, TickRate: 30, Ticks: 180, CheckpointEvery: 30}
	inputs, err := particleInputs([]ParticleInput{{Tick: 61, Actor: "presenter", DX: 20000, DY: -10000}})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := NewSimulationReplay(particleFactory(12), config, inputs)
	if err != nil {
		t.Fatal(err)
	}
	inputs[0].Data[0] = '!'
	for _, tick := range []int{0, 180, 61, 120, 29, 60, 61, 0, 179} {
		state, err := replay.Seek(tick)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(state, replay.frames[tick]) {
			t.Fatalf("seek to %d differs", tick)
		}
		state[0] = '!'
	}
	copy := replay.Checkpoints()
	copy[1].Snapshot[0] = '!'
	logged := replay.Inputs()
	logged[0].Data[0] = '!'
	if _, err := replay.Seek(61); err != nil {
		t.Fatalf("caller modified replay ownership: %v", err)
	}
	future, _ := particleInputs([]ParticleInput{{Tick: 61, Actor: "presenter", DX: 120000, DY: -40000}})
	branch, err := replay.Branch(60, future)
	if err != nil {
		t.Fatal(err)
	}
	for tick := 0; tick <= 60; tick++ {
		if !bytes.Equal(branch.frames[tick], replay.frames[tick]) {
			t.Fatalf("branch changed prefix at %d", tick)
		}
	}
	if bytes.Equal(branch.frames[120], replay.frames[120]) {
		t.Fatal("authored branch did not change future state")
	}
	again, err := NewSimulationReplay(particleFactory(12), config, replay.Inputs())
	if err != nil {
		t.Fatal(err)
	}
	left, _ := replay.StateHash(120)
	right, _ := again.StateHash(120)
	if left != right {
		t.Fatal("same seed and inputs changed state hash")
	}
	config.Seed++
	other, err := NewSimulationReplay(particleFactory(12), config, replay.Inputs())
	if err != nil {
		t.Fatal(err)
	}
	changed, _ := other.StateHash(120)
	if changed == left {
		t.Fatal("different explicit seeds should change this model")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, tick := range []int{61, 29, 120, 179, 0} {
				state, err := replay.Seek(tick)
				if err != nil || !bytes.Equal(state, replay.frames[tick]) {
					t.Errorf("concurrent absolute seek differed at %d: %v", tick, err)
				}
			}
		}()
	}
	wg.Wait()
}

type largeSimulation struct {
	size     int
	snapshot bool
}

func (s *largeSimulation) Tick(map[string]sim.Input) {}
func (s *largeSimulation) State() []byte {
	if s.snapshot {
		return nil
	}
	return make([]byte, s.size)
}
func (s *largeSimulation) Snapshot() []byte { return make([]byte, s.size) }
func (s *largeSimulation) Restore([]byte)   {}

type brokenSnapshotSimulation struct{ tick int }

func (s *brokenSnapshotSimulation) Tick(map[string]sim.Input) { s.tick++ }
func (s *brokenSnapshotSimulation) State() []byte             { value, _ := json.Marshal(s.tick); return value }
func (s *brokenSnapshotSimulation) Snapshot() []byte          { return []byte("0") }
func (s *brokenSnapshotSimulation) Restore([]byte)            { s.tick = 0 }

func TestSimulationReplayRejectsBrokenRestoreAndBounds(t *testing.T) {
	config := SimulationConfig{Seed: 0, TickRate: 30, Ticks: 60, CheckpointEvery: 30}
	_, err := NewSimulationReplay(func(SimulationConfig) (sim.Simulation, error) { return &brokenSnapshotSimulation{}, nil }, config, nil)
	if err == nil || !strings.Contains(err.Error(), "restore differs") {
		t.Fatalf("broken actual-state restore accepted: %v", err)
	}
	config.Ticks = 3601
	if _, err := NewSimulationReplay(particleFactory(1), config, nil); err == nil {
		t.Fatal("unbounded ticks accepted")
	}
	config.Ticks = 60
	if _, err := NewSimulationReplay(particleFactory(1), config, []SimulationInput{{Tick: 1, Actor: "x", Data: make([]byte, 4097)}}); err == nil {
		t.Fatal("oversized input accepted")
	}
	if _, err := NewSimulationReplay(particleFactory(1), config, []SimulationInput{{Tick: 1, Actor: "x"}, {Tick: 1, Actor: "x"}}); err == nil {
		t.Fatal("invisible input overwrite accepted")
	}
	replay, err := NewSimulationReplay(particleFactory(1), config, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = replay.Branch(30, []SimulationInput{{Tick: 30, Actor: "x"}}); err == nil {
		t.Fatal("branch changed immutable prefix")
	}
	if _, err = replay.Seek(-1); err == nil {
		t.Fatal("negative seek accepted")
	}
	if _, err = NewSimulationReplay(func(SimulationConfig) (sim.Simulation, error) { return &largeSimulation{size: 65537}, nil }, config, nil); err == nil || !strings.Contains(err.Error(), "64 KiB") {
		t.Fatalf("state byte limit not enforced: %v", err)
	}
	config.Ticks = 512
	config.CheckpointEvery = 1
	if _, err = NewSimulationReplay(func(SimulationConfig) (sim.Simulation, error) {
		return &largeSimulation{size: 65536, snapshot: true}, nil
	}, config, nil); err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("checkpoint budget not enforced: %v", err)
	}
}

func TestSimulationDeckNativeRenderingAndFiltering(t *testing.T) {
	deck, err := LoadIslandDeck("examples/simulation-lab")
	if err != nil {
		t.Fatal(err)
	}
	if len(deck.Simulations.Simulations) != 1 || len(deck.Simulations.Simulations[0].Branches) != 2 {
		t.Fatal("missing authored branches")
	}
	cd, err := compileDeckProgram(deck)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for _, node := range renderProgramSlides(island.NewRenderer("simulation-test"), deck, cd, nil) {
		out.WriteString(gosx.RenderHTML(node))
	}
	if !strings.Contains(out.String(), `data-simulation="demo"`) || strings.Count(out.String(), `data-simulation-particle=`) != 12 {
		t.Fatalf("native simulation SVG missing: %.400s", out.String())
	}
	if !strings.Contains(simulationAssets(deck), `"seed":"1729"`) || !strings.Contains(simulationAssets(deck), `"checkpoints":[0,30,60`) {
		t.Fatal("offline seed/checkpoint metadata missing")
	}
	original := deck.Simulations
	deck.Slides = []IslandSlide{deck.Slides[1], deck.Slides[0]}
	if err = FilterSimulations(deck); err != nil {
		t.Fatal(err)
	}
	if deck.Simulations.Simulations[0].Playhead[0].SlideIndex != 1 || original.Simulations[0].Playhead[0].SlideIndex != 0 {
		t.Fatal("stable-ID audience remap mutated its source")
	}
	deck.Slides = deck.Slides[:1]
	if err = FilterSimulations(deck); err != nil {
		t.Fatal(err)
	}
	if len(deck.Simulations.Simulations) != 0 || len(original.Simulations) != 1 {
		t.Fatal("audience filtering leaked or mutated omitted simulation")
	}
	plain, err := LoadIslandDeck("examples/theme-paper")
	if err != nil {
		t.Fatal(err)
	}
	if simulationAssets(plain) != "" {
		t.Fatal("plain deck loads simulation assets")
	}
}

func TestSimulationManifestRequiresExplicitSeedAndContainedPath(t *testing.T) {
	dir := t.TempDir()
	markdown, err := os.ReadFile("examples/simulation-lab/deck.md")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile("examples/simulation-lab/simulation.yaml")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "deck.md"), markdown, 0600)
	os.WriteFile(filepath.Join(dir, "simulation.yaml"), []byte(strings.Replace(string(manifest), "    seed: 1729\n", "", 1)), 0600)
	if _, err = LoadIslandDeck(dir); err == nil || !strings.Contains(err.Error(), "explicit seed") {
		t.Fatalf("missing seed accepted: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "simulation.yaml"), manifest, 0600)
	os.WriteFile(filepath.Join(dir, "deck.md"), []byte(strings.Replace(string(markdown), "simulation.yaml", "../escape.yaml", 1)), 0600)
	if _, err = LoadIslandDeck(dir); err == nil {
		t.Fatal("escaping manifest accepted")
	}
	// OpenRoot rejects an outside symlink even when its lexical name is local.
	os.WriteFile(filepath.Join(dir, "deck.md"), markdown, 0600)
	os.Remove(filepath.Join(dir, "simulation.yaml"))
	outside := filepath.Join(t.TempDir(), "simulation.yaml")
	os.WriteFile(outside, manifest, 0600)
	if err = os.Symlink(outside, filepath.Join(dir, "simulation.yaml")); err == nil {
		if _, err = LoadIslandDeck(dir); err == nil {
			t.Fatal("outside manifest symlink accepted")
		}
	}
}
