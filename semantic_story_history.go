package slides

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var architectureRevisionID = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,47}$`)

// ArchitectureSnapshot supplies explicit Sirena source, never a file or URL to
// fetch. ID is stable across regeneration; Label and Source are optional authored
// provenance returned in the report, and are not inserted into public captions.
type ArchitectureSnapshot struct {
	ID     string `json:"id" yaml:"id"`
	Label  string `json:"label,omitempty" yaml:"label,omitempty"`
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	Sirena []byte `json:"-" yaml:"-"`
}
type ArchitectureSnapshotInfo struct {
	ID     string `json:"id" yaml:"id"`
	Label  string `json:"label,omitempty" yaml:"label,omitempty"`
	Source string `json:"source,omitempty" yaml:"source,omitempty"`
	SHA256 string `json:"sha256" yaml:"sha256"`
}

// An explanation belongs to the incoming transition for its map key. Pointer
// captions distinguish a deliberately blank explanation from generated text.
// Optional FromID prevents silently reusing it after history is reordered.
type ArchitectureTourExplanation struct {
	FromID        string  `json:"from,omitempty" yaml:"from,omitempty"`
	BeforeCaption *string `json:"beforeCaption,omitempty" yaml:"beforeCaption,omitempty"`
	AfterCaption  *string `json:"afterCaption,omitempty" yaml:"afterCaption,omitempty"`
}
type ArchitectureTourCuration struct {
	Version      int                                    `json:"version" yaml:"version"`
	Explanations map[string]ArchitectureTourExplanation `json:"explanations" yaml:"explanations"`
}
type ArchitectureTourOptions struct {
	Title    string
	Curation *ArchitectureTourCuration
}
type ArchitectureTourTransition struct {
	From   string                 `json:"from"`
	To     string                 `json:"to"`
	Slide  string                 `json:"slide"`
	Change ArchitectureChangeTour `json:"change"`
}
type ArchitectureHistoryTour struct {
	Version              int                          `json:"version"`
	Snapshots            []ArchitectureSnapshotInfo   `json:"snapshots"`
	Transitions          []ArchitectureTourTransition `json:"transitions"`
	Curation             ArchitectureTourCuration     `json:"curation"`
	OrphanedExplanations []string                     `json:"orphanedExplanations"`
	Markdown             string                       `json:"markdown"`
	StoryYAML            string                       `json:"storyYAML"`
}

// ArchitectureTourHistory creates an ordered, bounded revision tour. It reuses
// Sirena's stable semantic IDs, not code heuristics. Regeneration replaces only
// generated output; explicit curation is copied unchanged (including orphaned
// entries), and predecessor mismatches fail for author reconciliation.
func ArchitectureTourHistory(snapshots []ArchitectureSnapshot, options ArchitectureTourOptions) (ArchitectureHistoryTour, error) {
	result := ArchitectureHistoryTour{Version: 1, Snapshots: []ArchitectureSnapshotInfo{}, Transitions: []ArchitectureTourTransition{}, OrphanedExplanations: []string{}, Curation: ArchitectureTourCuration{Version: 1, Explanations: map[string]ArchitectureTourExplanation{}}}
	if len(snapshots) < 2 || len(snapshots) > 32 {
		return result, fmt.Errorf("architecture history requires 2–32 explicit snapshots")
	}
	if len(options.Title) > 256 || !utf8.ValidString(options.Title) {
		return result, fmt.Errorf("tour title must be UTF-8 below 256 bytes")
	}
	title := options.Title
	if title == "" {
		title = "Architecture through time"
	}
	seen := map[string]bool{}
	total := 0
	for _, snapshot := range snapshots {
		if !architectureRevisionID.MatchString(snapshot.ID) || seen[snapshot.ID] {
			return result, fmt.Errorf("snapshot ID %q must be unique and 1–48 ASCII identifier characters", snapshot.ID)
		}
		if len(snapshot.Label) > 256 || len(snapshot.Source) > 1024 || !utf8.ValidString(snapshot.Label+snapshot.Source) || len(snapshot.Sirena) > maxSourceBytes || !utf8.Valid(snapshot.Sirena) {
			return result, fmt.Errorf("snapshot %s exceeds UTF-8 source/provenance limits", snapshot.ID)
		}
		total += len(snapshot.Sirena)
		if total > 4<<20 {
			return result, fmt.Errorf("architecture history source exceeds 4 MiB")
		}
		seen[snapshot.ID] = true
		hash := sha256.Sum256(snapshot.Sirena)
		result.Snapshots = append(result.Snapshots, ArchitectureSnapshotInfo{snapshot.ID, snapshot.Label, snapshot.Source, hex.EncodeToString(hash[:])})
	}
	if options.Curation != nil {
		if options.Curation.Version != 1 || len(options.Curation.Explanations) > 64 {
			return result, fmt.Errorf("tour curation needs version 1 and at most 64 explanations")
		}
		for id, explanation := range options.Curation.Explanations {
			if !architectureRevisionID.MatchString(id) || (explanation.FromID != "" && !architectureRevisionID.MatchString(explanation.FromID)) {
				return result, fmt.Errorf("invalid curated revision identity %q", id)
			}
			copy := explanation
			for _, slot := range []**string{&copy.BeforeCaption, &copy.AfterCaption} {
				if *slot == nil {
					continue
				}
				text := **slot
				if len(text) > 4096 || !utf8.ValidString(text) {
					return result, fmt.Errorf("curated caption for %s must be UTF-8 below 4096 bytes", id)
				}
				*slot = &text
			}
			result.Curation.Explanations[id] = copy
		}
	}
	manifest := struct {
		Version int         `yaml:"version"`
		Beats   []StoryBeat `yaml:"beats"`
	}{Version: 1, Beats: []StoryBeat{}}
	var markdown strings.Builder
	markdown.WriteString("---\ntitle: " + strconv.Quote(title) + "\nstory: story.yaml\noffline-required: true\n---\n\n")
	used := map[string]bool{}
	for i := 1; i < len(snapshots); i++ {
		before, after := snapshots[i-1], snapshots[i]
		change, err := ArchitectureTour(before.Sirena, after.Sirena)
		if err != nil {
			return result, fmt.Errorf("%s → %s: %w", before.ID, after.ID, err)
		}
		var pose struct {
			Version int         `yaml:"version"`
			Beats   []StoryBeat `yaml:"beats"`
		}
		if err := yaml.Unmarshal([]byte(change.StoryYAML), &pose); err != nil {
			return result, err
		}
		slide := "revision-" + after.ID
		if explanation, ok := result.Curation.Explanations[after.ID]; ok {
			if explanation.FromID != "" && explanation.FromID != before.ID {
				return result, fmt.Errorf("curated explanation for %s expects predecessor %s, received %s", after.ID, explanation.FromID, before.ID)
			}
			if explanation.BeforeCaption != nil {
				pose.Beats[0].Caption = *explanation.BeforeCaption
			}
			if explanation.AfterCaption != nil {
				pose.Beats[1].Caption = *explanation.AfterCaption
			}
			used[after.ID] = true
		}
		for j := range pose.Beats {
			pose.Beats[j].Slide = slide
		}
		manifest.Beats = append(manifest.Beats, pose.Beats...)
		encoded, err := yaml.Marshal(pose)
		if err != nil {
			return result, err
		}
		change.StoryYAML = string(encoded)
		if i > 1 {
			markdown.WriteString("\n---\n\n")
		}
		markdown.WriteString("```yaml\nid: " + slide + "\ncues: before, after\n```\n\n## Revision " + strconv.Itoa(i) + "\n\n:::diagram-morph {duration=700}\n" + storySnapshotFence(before.Sirena) + "\n" + storySnapshotFence(after.Sirena) + ":::\n\n<!-- Architecture transition; explicit explanations live in tour curation. -->\n")
		result.Transitions = append(result.Transitions, ArchitectureTourTransition{before.ID, after.ID, slide, change})
	}
	for id := range result.Curation.Explanations {
		if !used[id] {
			result.OrphanedExplanations = append(result.OrphanedExplanations, id)
		}
	}
	sort.Strings(result.OrphanedExplanations)
	encoded, err := yaml.Marshal(manifest)
	if err != nil {
		return result, err
	}
	result.Markdown, result.StoryYAML = markdown.String(), string(encoded)
	if len(result.Markdown) > 9<<20 {
		return result, fmt.Errorf("generated architecture history exceeds 9 MiB")
	}
	return result, nil
}

func storySnapshotFence(source []byte) string {
	longest, run := 0, 0
	for _, c := range source {
		if c == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	mark := strings.Repeat("`", max(3, longest+1))
	return mark + "sirena\n" + strings.TrimSpace(string(source)) + "\n" + mark + "\n"
}
