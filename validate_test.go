package slides

import "testing"

// TestValidateProfiles covers the per-profile authoring rules (previously 0%):
// conference requires notes on every slide; lecture requires 80% notes coverage;
// standard has no profile errors. A two-slide deck with no notes trips both
// strict profiles.
func TestValidateProfiles(t *testing.T) {
	noNotes := loadDeckFromSource(t, "---\naspect-ratio: 16:9\ncaption-safe-bottom: 20%\nduration-minutes: 25\noffline-required: true\n---\n\n# A\n\nsome words here\n\n---\n\n# B\n\nmore words\n", nil)

	if r := Validate(noNotes, ValidateOptions{Profile: "conference"}); r.Passed(false) {
		t.Errorf("conference profile should FAIL a deck with no notes; errors=%v", r.Errors)
	}
	if r := Validate(noNotes, ValidateOptions{Profile: "lecture"}); r.Passed(false) {
		t.Errorf("lecture profile should FAIL a deck under 80%% note coverage; errors=%v", r.Errors)
	}
	if r := Validate(noNotes, ValidateOptions{Profile: "standard"}); len(r.Errors) != 0 {
		t.Errorf("standard profile should have no errors, got %v", r.Errors)
	}

	// An unknown profile is reported as an error, not silently ignored.
	if r := Validate(noNotes, ValidateOptions{Profile: "bogus"}); r.Passed(false) {
		t.Errorf("unknown profile should error; errors=%v", r.Errors)
	}
}

func TestConferenceContract(t *testing.T) {
	missing := loadDeckFromSource(t, "# A\n\n<ParseTree/>\n\n<!-- fallback -->\n", map[string]string{"ParseTree.gsx": "component ParseTree() { <div>tree</div> }"})
	r := Validate(missing, ValidateOptions{Profile: "conference"})
	if len(r.Errors) < 5 {
		t.Fatalf("conference contract should report metadata and fallback errors: %v", r.Errors)
	}

	ready := loadDeckFromSource(t, "---\naspect-ratio: 16:9\ncaption-safe-bottom: 10%\nduration-minutes: 25\noffline-required: true\n---\n\n```yaml\nfallback: static\n```\n\n# A\n\n<ParseTree/>\n\n<!-- fallback -->\n", map[string]string{"ParseTree.gsx": "component ParseTree() { <div>tree</div> }"})
	if r := Validate(ready, ValidateOptions{Profile: "conference"}); len(r.Errors) != 0 {
		t.Fatalf("complete conference contract errors: %v", r.Errors)
	}
}
