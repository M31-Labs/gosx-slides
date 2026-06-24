package slides

import "strings"

// frontmatter.go holds the small, lane-agnostic helpers the real lane needs that
// used to live in the (now deleted) fallback parser: headmatter splitting, the
// key:value frontmatter parse, and a string de-dup. render_program.go and
// slidegen.go read deck/slide frontmatter through these.

// splitHeadmatter peels a leading `---`-delimited YAML headmatter block off the
// source, returning (headmatter, body, nil). With no headmatter it returns
// ("", src, nil).
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

// parseFrontmatter parses simple `key: value` lines (blank lines and `#` comments
// skipped, surrounding quotes trimmed) into a map. It is intentionally minimal —
// the deck headmatter and per-slide yaml fences are flat key/value blocks.
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

// uniqueStrings returns values with empties and duplicates removed, preserving
// first-seen order.
func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
