package slides

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
)

// Imported SVGs remain downloadable files, so image-element sandboxing alone
// is insufficient. Accept static SVG geometry and internal references only.
func migrationStaticSVG(content []byte) bool {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	tokens := 0
	root := false
	depth := 0
	allowed := map[string]bool{"svg": true, "g": true, "path": true, "rect": true, "circle": true, "ellipse": true, "line": true, "polyline": true, "polygon": true, "text": true, "tspan": true, "defs": true, "linearGradient": true, "radialGradient": true, "stop": true, "clipPath": true, "mask": true, "pattern": true, "title": true, "desc": true, "use": true, "symbol": true, "marker": true}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return root && depth == 0
		}
		if err != nil {
			return false
		}
		tokens++
		if tokens > 100000 {
			return false
		}
		switch token := token.(type) {
		case xml.Directive:
			return false
		case xml.ProcInst:
			if token.Target != "xml" {
				return false
			}
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(token)) != "" {
				return false
			}
		case xml.StartElement:
			if root && depth == 0 {
				return false
			}
			if !root {
				if token.Name.Local != "svg" {
					return false
				}
				root = true
			}
			if !allowed[token.Name.Local] {
				return false
			}
			depth++
			for _, attr := range token.Attr {
				key := strings.ToLower(attr.Name.Local)
				value := strings.ToLower(strings.TrimSpace(attr.Value))
				if key == "style" || strings.Contains(value, "\\") || strings.HasPrefix(key, "on") || key == "href" && !strings.HasPrefix(value, "#") || strings.Contains(value, "@import") || strings.Contains(value, "javascript:") {
					return false
				}
				for remaining := value; ; {
					_, tail, found := strings.Cut(remaining, "url(")
					if !found {
						break
					}
					url, after, closed := strings.Cut(tail, ")")
					if !closed || !strings.HasPrefix(strings.Trim(url, " '\""), "#") {
						return false
					}
					remaining = after
				}
			}
		case xml.EndElement:
			depth--
		}
	}
}
