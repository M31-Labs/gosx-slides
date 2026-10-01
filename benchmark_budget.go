package slides

import (
	"fmt"
	"math"
	"strings"
)

// Zero disables a ceiling. Browser timings depend on the machine; byte/DOM
// budgets and generous CI time ceilings catch gross regressions reproducibly.
type BrowserBudget struct {
	ReadyMillis    float64 `json:"readyMillis"`
	TransferBytes  int64   `json:"transferBytes"`
	DOMNodes       int64   `json:"domNodes"`
	HeapBytes      int64   `json:"heapBytes"`
	FrameP95Millis float64 `json:"frameP95Millis"`
}

func (b BrowserBudget) Validate() error {
	if b.ReadyMillis < 0 || b.FrameP95Millis < 0 || b.TransferBytes < 0 || b.DOMNodes < 0 || b.HeapBytes < 0 || math.IsNaN(b.ReadyMillis) || math.IsNaN(b.FrameP95Millis) || math.IsInf(b.ReadyMillis, 0) || math.IsInf(b.FrameP95Millis, 0) {
		return fmt.Errorf("benchmark ceilings must be finite and non-negative")
	}
	return nil
}
func (r *BrowserBenchmark) CheckBudget(b BrowserBudget) error {
	if err := b.Validate(); err != nil {
		return err
	}
	var failures []string
	for i, s := range r.Runs {
		checks := []struct {
			name          string
			actual, limit float64
		}{{"readyMillis", s.ReadyMillis, b.ReadyMillis}, {"transferBytes", float64(s.TransferBytes), float64(b.TransferBytes)}, {"domNodes", float64(s.DOMNodes), float64(b.DOMNodes)}, {"heapBytes", float64(s.HeapBytes), float64(b.HeapBytes)}, {"frameP95Millis", s.FrameP95Millis, b.FrameP95Millis}}
		for _, c := range checks {
			if c.limit > 0 && (c.actual > c.limit || math.IsNaN(c.actual) || math.IsInf(c.actual, 0) || c.actual <= 0) {
				failures = append(failures, fmt.Sprintf("run %d %s: %.3f exceeds %.3f (or measurement unavailable)", i+1, c.name, c.actual, c.limit))
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("browser budget failed: %s", strings.Join(failures, "; "))
	}
	return nil
}
