package slides

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var includeDirectiveRe = regexp.MustCompile(`\{\{include\s+(?:"([^"]+)"|'([^']+)')\s*\}\}`)

// expandIncludes replaces {{include "relative/path.md"}} directives.
func expandIncludes(src, baseDir string, stack map[string]bool) (string, []string, error) {
	if strings.TrimSpace(baseDir) == "" {
		return src, nil, nil
	}
	matches := includeDirectiveRe.FindAllStringSubmatchIndex(src, -1)
	if len(matches) == 0 {
		return src, nil, nil
	}

	var out strings.Builder
	var files []string
	last := 0
	for _, match := range matches {
		out.WriteString(src[last:match[0]])
		rel := ""
		if match[2] >= 0 {
			rel = src[match[2]:match[3]]
		} else if match[4] >= 0 {
			rel = src[match[4]:match[5]]
		}
		included, nested, err := readInclude(rel, baseDir, stack)
		if err != nil {
			return "", nil, err
		}
		files = append(files, nested...)
		out.WriteString(included)
		last = match[1]
	}
	out.WriteString(src[last:])
	return out.String(), uniqueStrings(files), nil
}

func readInclude(rel, baseDir string, stack map[string]bool) (string, []string, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("include %q must stay inside the deck tree", rel)
	}
	path := filepath.Join(baseDir, clean)
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if stack[abs] {
		return "", nil, fmt.Errorf("include cycle detected at %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("include %s: %w", path, err)
	}
	stack[abs] = true
	expanded, nested, err := expandIncludes(normalizeNewlines(string(data)), filepath.Dir(path), stack)
	delete(stack, abs)
	if err != nil {
		return "", nil, err
	}
	files := append([]string{path}, nested...)
	return strings.TrimSpace(expanded), files, nil
}

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
