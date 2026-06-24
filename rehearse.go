package slides

import "fmt"

// formatSeconds renders a second count as "MmSSs" for run sheets and estimates.
func formatSeconds(total int) string {
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%dm%02ds", total/60, total%60)
}
