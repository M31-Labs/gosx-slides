package slides

import (
	"sort"
	"strconv"
	"strings"
)

// LineStep describes one click step in a code walkthrough.
type LineStep struct {
	All   bool
	Lines []int
}

// ParseRangeSpec parses strings like "1-3|5|all" into click steps.
func ParseRangeSpec(spec string) []LineStep {
	spec = strings.TrimSpace(spec)
	spec = strings.TrimPrefix(spec, "{")
	spec = strings.TrimSuffix(spec, "}")
	if spec == "" {
		return nil
	}
	parts := strings.Split(spec, "|")
	steps := make([]LineStep, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.EqualFold(part, "all") {
			steps = append(steps, LineStep{All: true})
			continue
		}
		seen := map[int]bool{}
		for _, token := range strings.Split(part, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if strings.Contains(token, "-") {
				pair := strings.SplitN(token, "-", 2)
				start, startErr := strconv.Atoi(strings.TrimSpace(pair[0]))
				end, endErr := strconv.Atoi(strings.TrimSpace(pair[1]))
				if startErr != nil || endErr != nil {
					continue
				}
				if end < start {
					start, end = end, start
				}
				for n := start; n <= end; n++ {
					if n > 0 {
						seen[n] = true
					}
				}
				continue
			}
			n, err := strconv.Atoi(token)
			if err == nil && n > 0 {
				seen[n] = true
			}
		}
		lines := make([]int, 0, len(seen))
		for n := range seen {
			lines = append(lines, n)
		}
		sort.Ints(lines)
		steps = append(steps, LineStep{Lines: lines})
	}
	return steps
}

func rangeSpecFromInfo(info string) string {
	start := strings.Index(info, "{")
	end := strings.LastIndex(info, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return strings.TrimSpace(info[start : end+1])
}

func lineSteps(line int, steps []LineStep) string {
	if len(steps) == 0 {
		return ""
	}
	var matches []string
	for i, step := range steps {
		if step.All {
			matches = append(matches, strconv.Itoa(i+1))
			continue
		}
		for _, n := range step.Lines {
			if n == line {
				matches = append(matches, strconv.Itoa(i+1))
				break
			}
		}
	}
	return strings.Join(matches, ",")
}
