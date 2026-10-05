package slides

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"m31labs.dev/gosx/sim"
	"m31labs.dev/mdpp"
)

type SimulationPose struct {
	Slide      string `json:"slide" yaml:"slide"`
	Cue        string `json:"cue" yaml:"cue"`
	Tick       int    `json:"tick" yaml:"tick"`
	EndTick    *int   `json:"endTick,omitempty" yaml:"end-tick,omitempty"`
	SlideIndex int    `json:"slideIndex" yaml:"-"`
	Step       int    `json:"step" yaml:"-"`
}
type ParticleInput struct {
	Tick  int    `json:"tick" yaml:"tick"`
	Actor string `json:"actor" yaml:"actor"`
	DX    int    `json:"dx" yaml:"dx"`
	DY    int    `json:"dy" yaml:"dy"`
}
type SimulationBranchSpec struct {
	ID       string          `yaml:"id"`
	Label    string          `yaml:"label"`
	ForkTick int             `yaml:"fork-tick"`
	Inputs   []ParticleInput `yaml:"inputs"`
}
type SimulationSpec struct {
	ID              string                 `yaml:"id"`
	Label           string                 `yaml:"label"`
	Seed            *uint64                `yaml:"seed"`
	TickRate        int                    `yaml:"tick-rate"`
	Ticks           int                    `yaml:"ticks"`
	CheckpointEvery int                    `yaml:"checkpoint-every"`
	Particles       int                    `yaml:"particles"`
	Inputs          []ParticleInput        `yaml:"inputs"`
	Branches        []SimulationBranchSpec `yaml:"branches"`
	Playhead        []SimulationPose       `yaml:"playhead"`
}
type SimulationFrame struct {
	Hash  string          `json:"hash"`
	State json.RawMessage `json:"state"`
}
type CompiledSimulationBranch struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	ForkTick    int               `json:"forkTick"`
	Inputs      []ParticleInput   `json:"inputs"`
	Frames      []SimulationFrame `json:"frames"`
	Checkpoints []int             `json:"checkpoints"`
}
type CompiledSimulation struct {
	ID       string                     `json:"id"`
	Label    string                     `json:"label"`
	Seed     string                     `json:"seed"` // decimal string preserves all uint64 seeds in JS
	TickRate int                        `json:"tickRate"`
	Ticks    int                        `json:"ticks"`
	Playhead []SimulationPose           `json:"playhead"`
	Branches []CompiledSimulationBranch `json:"branches"`
}
type CompiledSimulations struct {
	Version     int                  `json:"version"`
	Simulations []CompiledSimulation `json:"simulations"`
}

// CompileSimulations compiles one bounded, local manifest. The intentionally
// small particle model demonstrates the generic GoSX replay adapter; it is not
// a general physics language. All transitions and inputs must be authored.
func CompileSimulations(deck *IslandDeck) (*CompiledSimulations, error) {
	if deck == nil {
		return nil, fmt.Errorf("simulation requires a deck")
	}
	file := deckFrontmatterString(deck, "simulation")
	if file == "" {
		return nil, nil
	}
	if !safeDeckRelPath(file) {
		return nil, fmt.Errorf("simulation manifest must stay within the deck directory")
	}
	root, err := os.OpenRoot(deck.Dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	f, err := root.Open(file)
	if err != nil {
		return nil, fmt.Errorf("simulation: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("simulation manifest must be a regular file below 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, fmt.Errorf("simulation manifest exceeds 1 MiB")
	}
	var manifest struct {
		Version     int              `yaml:"version"`
		Simulations []SimulationSpec `yaml:"simulations"`
	}
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.KnownFields(true)
	if err = d.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("simulation: %w", err)
	}
	if d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("simulation expects one YAML document")
	}
	if manifest.Version != 1 || len(manifest.Simulations) < 1 || len(manifest.Simulations) > 8 {
		return nil, fmt.Errorf("simulation requires version 1 and 1–8 simulations")
	}
	compiled := &CompiledSimulations{Version: 1}
	ids := map[string]bool{}
	for _, spec := range manifest.Simulations {
		if !slideClassTokenRe.MatchString(spec.ID) || ids[spec.ID] {
			return nil, fmt.Errorf("simulation ID must be a unique CSS token")
		}
		ids[spec.ID] = true
		if spec.Seed == nil || spec.Particles < 1 || spec.Particles > 32 || len(spec.Label) > 256 || len(spec.Branches) > 3 || len(spec.Playhead) > 128 {
			return nil, fmt.Errorf("simulation %s needs an explicit seed, 1–32 particles, at most 3 branches and 128 poses", spec.ID)
		}
		config := SimulationConfig{Seed: *spec.Seed, TickRate: spec.TickRate, Ticks: spec.Ticks, CheckpointEvery: spec.CheckpointEvery}
		factory := particleFactory(spec.Particles)
		inputs, err := particleInputs(spec.Inputs)
		if err != nil {
			return nil, fmt.Errorf("simulation %s: %w", spec.ID, err)
		}
		replay, err := NewSimulationReplay(factory, config, inputs)
		if err != nil {
			return nil, fmt.Errorf("simulation %s: %w", spec.ID, err)
		}
		label := spec.Label
		if label == "" {
			label = spec.ID
		}
		entry := CompiledSimulation{ID: spec.ID, Label: label, Seed: fmt.Sprint(*spec.Seed), TickRate: config.TickRate, Ticks: config.Ticks}
		seen := map[string]bool{}
		for _, pose := range spec.Playhead {
			if deck.storyExcludedSlides[pose.Slide] {
				continue
			}
			pose.SlideIndex = -1
			pose.Step = -1
			for i, slide := range deck.Slides {
				id, _ := slideFrontmatterValues(slide)["id"].(string)
				if id != pose.Slide {
					continue
				}
				if pose.SlideIndex >= 0 {
					return nil, fmt.Errorf("simulation %s: ambiguous slide ID %q", spec.ID, id)
				}
				pose.SlideIndex = i
				for step, cue := range slideCueNames(slide) {
					if cue == pose.Cue {
						pose.Step = step
					}
				}
			}
			key := pose.Slide + "/" + pose.Cue
			if pose.SlideIndex < 0 || pose.Step < 0 || seen[key] || pose.Tick < 0 || pose.Tick > config.Ticks || pose.EndTick != nil && (*pose.EndTick < pose.Tick || *pose.EndTick > config.Ticks) {
				return nil, fmt.Errorf("simulation %s has an invalid or duplicate pose %s", spec.ID, key)
			}
			seen[key] = true
			entry.Playhead = append(entry.Playhead, pose)
		}
		base, err := compileSimulationBranch(replay, "baseline", "Baseline", 0)
		if err != nil {
			return nil, err
		}
		entry.Branches = append(entry.Branches, base)
		branchIDs := map[string]bool{"baseline": true}
		for _, branch := range spec.Branches {
			if !slideClassTokenRe.MatchString(branch.ID) || branchIDs[branch.ID] || len(branch.Label) > 256 {
				return nil, fmt.Errorf("simulation %s has an invalid or duplicate branch ID", spec.ID)
			}
			branchIDs[branch.ID] = true
			future, err := particleInputs(branch.Inputs)
			if err != nil {
				return nil, err
			}
			fork, err := replay.Branch(branch.ForkTick, future)
			if err != nil {
				return nil, err
			}
			label := branch.Label
			if label == "" {
				label = branch.ID
			}
			compiledBranch, err := compileSimulationBranch(fork, branch.ID, label, branch.ForkTick)
			if err != nil {
				return nil, err
			}
			entry.Branches = append(entry.Branches, compiledBranch)
			// Stop at the output budget after every bounded branch, instead of
			// allowing a full multi-branch entry to grow beyond it first.
			encoded, _ := json.Marshal(entry)
			if len(encoded) > maxSimulationBytes {
				return nil, fmt.Errorf("compiled simulation exceeds 16 MiB")
			}
		}
		compiled.Simulations = append(compiled.Simulations, entry)
		encoded, _ := json.Marshal(compiled)
		if len(encoded) > maxSimulationBytes {
			return nil, fmt.Errorf("compiled simulation deck exceeds 16 MiB")
		}
	}
	return compiled, nil
}

func compileSimulationBranch(replay *SimulationReplay, id, label string, forkTick int) (CompiledSimulationBranch, error) {
	branch := CompiledSimulationBranch{ID: id, Label: label, ForkTick: forkTick, Inputs: []ParticleInput{}}
	for _, input := range replay.Inputs() {
		var impulse particleImpulse
		if err := json.Unmarshal(input.Data, &impulse); err != nil {
			return branch, err
		}
		branch.Inputs = append(branch.Inputs, ParticleInput{Tick: input.Tick, Actor: input.Actor, DX: impulse.DX, DY: impulse.DY})
	}
	for _, point := range replay.Checkpoints() {
		branch.Checkpoints = append(branch.Checkpoints, point.Tick)
	}
	for _, state := range replay.frames {
		branch.Frames = append(branch.Frames, SimulationFrame{State: bytes.Clone(state), Hash: fmt.Sprintf("%x", sha256.Sum256(state))})
	}
	return branch, nil
}

func simulationMounts(deck *IslandDeck) map[string]bool {
	ids := map[string]bool{}
	var walk func(*mdpp.Node)
	walk = func(n *mdpp.Node) {
		if n.Type == mdpp.NodeContainerDirective && n.Attr("name") == "simulation" {
			ids[strings.TrimSpace(n.Attr("title"))] = true
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	for _, slide := range deck.Slides {
		walk(slide.Node)
	}
	return ids
}

// FilterSimulations deep-copies and prunes public simulation data after an
// audience selection, remapping poses by stable authored slide ID.
func FilterSimulations(deck *IslandDeck) error {
	if deck.Simulations == nil {
		return nil
	}
	encoded, err := json.Marshal(deck.Simulations)
	if err != nil {
		return err
	}
	copy := new(CompiledSimulations)
	if err = json.Unmarshal(encoded, copy); err != nil {
		return err
	}
	mounts := simulationMounts(deck)
	ids := map[string]int{}
	for i, slide := range deck.Slides {
		id, _ := slideFrontmatterValues(slide)["id"].(string)
		if id != "" {
			ids[id] = i
		}
	}
	retained := []CompiledSimulation{}
	for _, entry := range copy.Simulations {
		if !mounts[entry.ID] {
			continue
		}
		poses := []SimulationPose{}
		for _, pose := range entry.Playhead {
			if i, ok := ids[pose.Slide]; ok {
				pose.SlideIndex = i
				poses = append(poses, pose)
			}
		}
		entry.Playhead = poses
		retained = append(retained, entry)
	}
	copy.Simulations = retained
	deck.Simulations = copy
	return prepareSimulationMounts(deck)
}

func attachSimulations(deck *IslandDeck) error {
	compiled, err := CompileSimulations(deck)
	if err != nil {
		return err
	}
	deck.Simulations = compiled
	known := map[string]bool{}
	if compiled != nil {
		for _, entry := range compiled.Simulations {
			known[entry.ID] = true
		}
	}
	for id := range simulationMounts(deck) {
		if !known[id] {
			return fmt.Errorf("simulation directive %q has no manifest entry", id)
		}
	}
	return FilterSimulations(deck)
}

type particleImpulse struct {
	DX int `json:"dx"`
	DY int `json:"dy"`
}
type simulationParticle struct {
	X  int `json:"x"`
	Y  int `json:"y"`
	VX int `json:"vx"`
	VY int `json:"vy"`
}
type particleState struct {
	Tick      int                  `json:"tick"`
	Random    uint64               `json:"random,string"`
	Particles []simulationParticle `json:"particles"`
}
type particleSimulation struct {
	state particleState
	rate  int
}

var _ sim.Simulation = (*particleSimulation)(nil)

func particleInputs(inputs []ParticleInput) ([]SimulationInput, error) {
	out := []SimulationInput{}
	for _, input := range inputs {
		if input.DX < -200000 || input.DX > 200000 || input.DY < -200000 || input.DY > 200000 {
			return nil, fmt.Errorf("particle impulse must be within +/-200000 milli-pixels/second")
		}
		data, _ := json.Marshal(particleImpulse{DX: input.DX, DY: input.DY})
		out = append(out, SimulationInput{Tick: input.Tick, Actor: input.Actor, Data: data})
	}
	return out, nil
}
func particleFactory(count int) SimulationFactory {
	return func(config SimulationConfig) (sim.Simulation, error) {
		p := &particleSimulation{rate: config.TickRate, state: particleState{Random: config.Seed}}
		if p.state.Random == 0 {
			p.state.Random = 0x9e3779b97f4a7c15
		}
		for i := 0; i < count; i++ {
			p.state.Particles = append(p.state.Particles, simulationParticle{X: 30000 + int(p.random()%540000), Y: 30000 + int(p.random()%240000), VX: int(p.random()%120001) - 60000, VY: int(p.random()%80001) - 40000})
		}
		return p, nil
	}
}
func (p *particleSimulation) random() uint64 {
	x := p.state.Random
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	p.state.Random = x
	return x
}
func (p *particleSimulation) Tick(inputs map[string]sim.Input) {
	actors := make([]string, 0, len(inputs))
	for actor := range inputs {
		actors = append(actors, actor)
	}
	sort.Strings(actors)
	for _, actor := range actors {
		var impulse particleImpulse
		if json.Unmarshal(inputs[actor].Data, &impulse) != nil {
			continue
		}
		for i := range p.state.Particles {
			particle := &p.state.Particles[i]
			particle.VX = max(-200000, min(200000, particle.VX+impulse.DX))
			particle.VY = max(-200000, min(200000, particle.VY+impulse.DY))
		}
	}
	for i := range p.state.Particles {
		v := &p.state.Particles[i]
		v.X += v.VX / p.rate
		v.Y += v.VY / p.rate
		if v.X < 12000 {
			v.X = 24000 - v.X
			v.VX = -v.VX
		}
		if v.X > 588000 {
			v.X = 1176000 - v.X
			v.VX = -v.VX
		}
		if v.Y < 12000 {
			v.Y = 24000 - v.Y
			v.VY = -v.VY
		}
		if v.Y > 288000 {
			v.Y = 576000 - v.Y
			v.VY = -v.VY
		}
	}
	p.state.Tick++
}
func (p *particleSimulation) Snapshot() []byte { data, _ := json.Marshal(p.state); return data }
func (p *particleSimulation) State() []byte    { return p.Snapshot() }
func (p *particleSimulation) Restore(data []byte) {
	var state particleState
	if json.Unmarshal(data, &state) == nil {
		p.state = state
	}
}
