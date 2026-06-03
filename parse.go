package slides

import (
	"regexp"
	"strconv"
	"strings"
)

type rawSlide struct {
	Frontmatter string
	Body        string
	SourcePath  string
}

func splitHeadmatter(src string) (string, string, error) {
	lines := strings.SplitAfter(src, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", src, nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			head := strings.Join(lines[1:i], "")
			body := strings.Join(lines[i+1:], "")
			return head, body, nil
		}
	}
	return "", src, nil
}

func splitSlideSections(src string) ([]rawSlide, error) {
	lines := strings.SplitAfter(src, "\n")
	var slides []rawSlide
	var body strings.Builder
	var frontmatter string
	inFence := false
	fenceMarker := ""

	flush := func() {
		if strings.TrimSpace(body.String()) == "" && strings.TrimSpace(frontmatter) == "" {
			body.Reset()
			frontmatter = ""
			return
		}
		slides = append(slides, rawSlide{
			Frontmatter: strings.TrimSpace(frontmatter),
			Body:        strings.TrimSpace(body.String()),
		})
		body.Reset()
		frontmatter = ""
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if isFenceStart(trimmed) {
			marker := fencePrefix(trimmed)
			if !inFence {
				inFence = true
				fenceMarker = marker
			} else if strings.HasPrefix(trimmed, fenceMarker) {
				inFence = false
				fenceMarker = ""
			}
			body.WriteString(line)
			continue
		}
		if !inFence && trimmed == "---" {
			flush()
			if fm, end, ok := readSlideFrontmatter(lines, i+1); ok {
				frontmatter = fm
				i = end
			}
			continue
		}
		body.WriteString(line)
	}
	flush()
	return slides, nil
}

func splitSlideFileSections(src, sourcePath string) ([]rawSlide, error) {
	src = normalizeNewlines(src)
	if startsWithFrontmatter(src) {
		frontmatter, body, err := splitHeadmatter(src)
		if err != nil {
			return nil, err
		}
		slides, err := splitSlideSections(body)
		if err != nil {
			return nil, err
		}
		if len(slides) == 0 {
			slides = []rawSlide{{Frontmatter: frontmatter, Body: body}}
		} else if strings.TrimSpace(slides[0].Frontmatter) == "" {
			slides[0].Frontmatter = frontmatter
		} else {
			slides = append([]rawSlide{{Frontmatter: frontmatter}}, slides...)
		}
		for i := range slides {
			slides[i].SourcePath = sourcePath
		}
		return slides, nil
	}
	slides, err := splitSlideSections(src)
	if err != nil {
		return nil, err
	}
	for i := range slides {
		slides[i].SourcePath = sourcePath
	}
	return slides, nil
}

func startsWithFrontmatter(src string) bool {
	lines := strings.SplitN(src, "\n", 2)
	return len(lines) > 0 && strings.TrimSpace(lines[0]) == "---"
}

func readSlideFrontmatter(lines []string, start int) (string, int, bool) {
	if start >= len(lines) {
		return "", start, false
	}
	var block strings.Builder
	hasKey := false
	for i := start; i < len(lines) && i < start+80; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			if hasKey {
				return block.String(), i, true
			}
			return "", start, false
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			block.WriteString(lines[i])
			continue
		}
		if isFrontmatterLine(trimmed) {
			hasKey = true
			block.WriteString(lines[i])
			continue
		}
		return "", start, false
	}
	return "", start, false
}

func isFrontmatterLine(line string) bool {
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return false
	}
	key := strings.TrimSpace(line[:idx])
	if key == "" {
		return false
	}
	for _, r := range key {
		if !(r == '-' || r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func parseFrontmatter(src string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		value = strings.Trim(value, `"'`)
		out[key] = value
	}
	return out
}

func extractNotes(src string) (string, string) {
	var notes []string
	notesBlock := regexp.MustCompile(`(?is)<Notes>(.*?)</Notes>`)
	src = notesBlock.ReplaceAllStringFunc(src, func(match string) string {
		m := notesBlock.FindStringSubmatch(match)
		if len(m) == 2 {
			notes = append(notes, strings.TrimSpace(m[1]))
		}
		return ""
	})
	comment := regexp.MustCompile(`(?s)<!--(.*?)-->`)
	src = comment.ReplaceAllStringFunc(src, func(match string) string {
		m := comment.FindStringSubmatch(match)
		if len(m) == 2 {
			notes = append(notes, strings.TrimSpace(m[1]))
		}
		return ""
	})
	return strings.TrimSpace(src), strings.Join(notes, "\n\n")
}

func firstHeading(src string) string {
	for _, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if level := headingLevel(trimmed); level > 0 {
			return strings.TrimSpace(trimmed[level:])
		}
	}
	return ""
}

func inferClicks(src string) int {
	maxClick := 0
	stepRe := regexp.MustCompile(`(?is)<Step\s+[^>]*\bn\s*=\s*["{]?([0-9]+)["}]?`)
	for _, match := range stepRe.FindAllStringSubmatch(src, -1) {
		if len(match) != 2 {
			continue
		}
		n, _ := strconv.Atoi(match[1])
		if n > maxClick {
			maxClick = n
		}
	}
	for _, spec := range codeRangeSpecs(src) {
		if steps := len(ParseRangeSpec(spec)); steps > maxClick {
			maxClick = steps
		}
	}
	return maxClick
}

func codeRangeSpecs(src string) []string {
	var specs []string
	lines := strings.Split(src, "\n")
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inFence && isFenceStart(trimmed) {
			inFence = true
			if spec := rangeSpecFromInfo(strings.TrimLeft(trimmed, "`~")); spec != "" {
				specs = append(specs, spec)
			}
			continue
		}
		if inFence && isFenceStart(trimmed) {
			inFence = false
		}
	}
	return specs
}

func isFenceStart(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}

func fencePrefix(trimmed string) string {
	if strings.HasPrefix(trimmed, "~~~") {
		return "~~~"
	}
	return "```"
}
