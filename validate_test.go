package slides

import "testing"

// TestValidateProfiles covers the per-profile authoring rules (previously 0%):
// conference requires notes on every slide; lecture requires 80% notes coverage;
// standard has no profile errors. A two-slide deck with no notes trips both
// strict profiles.
func TestValidateProfiles(t *testing.T) {
	noNotes := loadDeckFromSource(t, "# A\n\nsome words here\n\n---\n\n# B\n\nmore words\n", nil)

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
