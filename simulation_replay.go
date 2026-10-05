package slides

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"unicode/utf8"

	"m31labs.dev/gosx/sim"
)

// SimulationConfig owns the seed and fixed timestep. A factory must return a
// fresh deterministic simulation; Tick advances exactly 1/TickRate seconds.
type SimulationConfig struct {
	Seed            uint64
	TickRate        int
	Ticks           int
	CheckpointEvery int
}
type SimulationFactory func(SimulationConfig) (sim.Simulation, error)

// SimulationInput is an authored input applied before Tick advances to Tick.
// There is at most one input per actor per tick, as in GoSX's input collector.
type SimulationInput struct {
	Tick  int    `json:"tick"`
	Actor string `json:"actor"`
	Data  []byte `json:"data"`
}
type SimulationCheckpoint struct {
	Tick     int
	Snapshot []byte
}

// SimulationReplay adapts the native GoSX simulation contract to bounded,
// absolute presentation seeking, without Runner's wall-clock loop or unbounded
// recorder. It owns copied inputs, checkpoints and reference state frames.
type SimulationReplay struct {
	mu          sync.Mutex  // serializes seeks on this replay
	factoryMu   *sync.Mutex // all descendants share the factory resource guard
	config      SimulationConfig
	factory     SimulationFactory
	inputs      []SimulationInput
	checkpoints []SimulationCheckpoint
	frames      [][]byte
}

const maxSimulationBytes = 16 << 20

func NewSimulationReplay(factory SimulationFactory, config SimulationConfig, inputs []SimulationInput) (*SimulationReplay, error) {
	return newSimulationReplay(factory, config, inputs, new(sync.Mutex))
}

func newSimulationReplay(factory SimulationFactory, config SimulationConfig, inputs []SimulationInput, factoryMu *sync.Mutex) (*SimulationReplay, error) {
	if factory == nil || config.TickRate < 1 || config.TickRate > 120 || config.Ticks < 1 || config.Ticks > 3600 || config.CheckpointEvery < 1 || config.CheckpointEvery > 240 {
		return nil, fmt.Errorf("simulation needs a factory, 1–120 ticks/second, 1–3600 ticks and 1–240 checkpoint spacing")
	}
	if len(inputs) > 512 {
		return nil, fmt.Errorf("simulation supports at most 512 logged inputs")
	}
	r := &SimulationReplay{config: config, factory: factory, factoryMu: factoryMu}
	for _, input := range inputs {
		if input.Tick < 1 || input.Tick > config.Ticks || len(input.Actor) == 0 || len(input.Actor) > 64 || !utf8.ValidString(input.Actor) || len(input.Data) > 4096 {
			return nil, fmt.Errorf("simulation input has an invalid tick, actor or payload (maximum 4 KiB)")
		}
		input.Data = bytes.Clone(input.Data)
		r.inputs = append(r.inputs, input)
	}
	sort.Slice(r.inputs, func(i, j int) bool {
		if r.inputs[i].Tick == r.inputs[j].Tick {
			return r.inputs[i].Actor < r.inputs[j].Actor
		}
		return r.inputs[i].Tick < r.inputs[j].Tick
	})
	for i := 1; i < len(r.inputs); i++ {
		if r.inputs[i].Tick == r.inputs[i-1].Tick && r.inputs[i].Actor == r.inputs[i-1].Actor {
			return nil, fmt.Errorf("duplicate simulation input for actor %q at tick %d", r.inputs[i].Actor, r.inputs[i].Tick)
		}
	}
	s, err := r.newModel()
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("simulation factory returned nil")
	}
	budget := 0
	for tick := 0; tick <= config.Ticks; tick++ {
		if tick > 0 {
			s.Tick(r.tickInputs(tick))
		}
		state := bytes.Clone(s.State())
		if len(state) > 64<<10 {
			return nil, fmt.Errorf("simulation state exceeds 64 KiB at tick %d", tick)
		}
		budget += len(state)
		r.frames = append(r.frames, state)
		if tick%config.CheckpointEvery == 0 {
			snapshot := bytes.Clone(s.Snapshot())
			if len(snapshot) > 64<<10 {
				return nil, fmt.Errorf("simulation checkpoint exceeds 64 KiB at tick %d", tick)
			}
			budget += len(snapshot)
			r.checkpoints = append(r.checkpoints, SimulationCheckpoint{Tick: tick, Snapshot: snapshot})
		}
		if budget > maxSimulationBytes {
			return nil, fmt.Errorf("simulation frames/checkpoints exceed 16 MiB")
		}
	}
	// Catch a snapshot implementation that does not restore the actual state.
	if _, err := r.Seek(config.Ticks); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *SimulationReplay) newModel() (sim.Simulation, error) {
	r.factoryMu.Lock()
	defer r.factoryMu.Unlock()
	return r.factory(r.config)
}

func (r *SimulationReplay) tickInputs(tick int) map[string]sim.Input {
	inputs := map[string]sim.Input{}
	for _, input := range r.inputs {
		if input.Tick > tick {
			break
		}
		if input.Tick == tick {
			inputs[input.Actor] = sim.Input{Data: bytes.Clone(input.Data)}
		}
	}
	return inputs
}

// Seek restores the closest checkpoint and replays exact logged inputs. The
// reference frame detects broken restore or nondeterministic simulation code.
func (r *SimulationReplay) Seek(tick int) ([]byte, error) {
	if tick < 0 || tick > r.config.Ticks {
		return nil, fmt.Errorf("simulation tick outside 0–%d", r.config.Ticks)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, err := r.newModel()
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("simulation factory returned nil")
	}
	if !bytes.Equal(s.State(), r.frames[0]) {
		return nil, fmt.Errorf("simulation factory does not reproduce seeded initial state")
	}
	checkpoint := r.checkpoints[tick/r.config.CheckpointEvery]
	s.Restore(bytes.Clone(checkpoint.Snapshot))
	if !bytes.Equal(s.State(), r.frames[checkpoint.Tick]) {
		return nil, fmt.Errorf("simulation restore differs at checkpoint %d", checkpoint.Tick)
	}
	for frame := checkpoint.Tick + 1; frame <= tick; frame++ {
		s.Tick(r.tickInputs(frame))
	}
	state := s.State()
	if !bytes.Equal(state, r.frames[tick]) {
		return nil, fmt.Errorf("simulation replay differs at tick %d", tick)
	}
	return bytes.Clone(state), nil
}

// Branch preserves the prefix through tick and replaces all future inputs.
// The original replay remains immutable, so returning to it restores its state.
func (r *SimulationReplay) Branch(tick int, future []SimulationInput) (*SimulationReplay, error) {
	if tick < 0 || tick > r.config.Ticks {
		return nil, fmt.Errorf("simulation branch tick outside range")
	}
	inputs := []SimulationInput{}
	for _, input := range r.inputs {
		if input.Tick <= tick {
			inputs = append(inputs, input)
		}
	}
	for _, input := range future {
		if input.Tick <= tick {
			return nil, fmt.Errorf("branch inputs must follow fork tick %d", tick)
		}
		inputs = append(inputs, input)
	}
	branch, err := newSimulationReplay(r.factory, r.config, inputs, r.factoryMu)
	if err != nil {
		return nil, err
	}
	for frame := 0; frame <= tick; frame++ {
		if !bytes.Equal(branch.frames[frame], r.frames[frame]) {
			return nil, fmt.Errorf("simulation branch changed prefix at tick %d", frame)
		}
	}
	return branch, nil
}

func (r *SimulationReplay) Inputs() []SimulationInput {
	inputs := append([]SimulationInput(nil), r.inputs...)
	for i := range inputs {
		inputs[i].Data = bytes.Clone(inputs[i].Data)
	}
	return inputs
}
func (r *SimulationReplay) Checkpoints() []SimulationCheckpoint {
	points := append([]SimulationCheckpoint(nil), r.checkpoints...)
	for i := range points {
		points[i].Snapshot = bytes.Clone(points[i].Snapshot)
	}
	return points
}
func (r *SimulationReplay) StateHash(tick int) (string, error) {
	state, err := r.Seek(tick)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(state)), nil
}
