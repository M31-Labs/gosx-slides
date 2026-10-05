package slides

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

const snapshotAssetLimit = 16 << 20
const snapshotTotalLimit = 96 << 20
const snapshotCSSDepthLimit = 8

var snapshotCSSURL = regexp.MustCompile(`(?i)url\(\s*(?:"([^"]*)"|'([^']*)'|([^)]*?))\s*\)`)
var snapshotCSSImport = regexp.MustCompile(`(?i)^(@import\s+)(?:"([^"]*)"|'([^']*)')`)
var snapshotCSSURLAtStart = regexp.MustCompile("^" + snapshotCSSURL.String())

// Inline only published local resources. External URLs remain author-controlled;
// snapshotting never fetches an arbitrary server or reads outside the deck.
func inlineSnapshotAssets(deck *IslandDeck, document string) (string, error) {
	root, err := filepath.EvalSymlinks(deck.Dir)
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return "", err
	}
	defer directory.Close()
	cache := map[string]string{}
	pending := map[string]bool{}
	total := 0
	expanded := 0
	var css func(string, string, int) (string, error)
	var inline func(string, string, int) (string, error)
	inline = func(raw, base string, depth int) (string, error) {
		parsed, err := url.Parse(raw)
		if err != nil {
			return raw, nil
		}
		if parsed.IsAbs() || parsed.Host != "" || parsed.Path == "" {
			return raw, nil
		}
		path := strings.TrimPrefix(parsed.Path, "/")
		if !strings.HasPrefix(parsed.Path, "/") && base != "" {
			path = pathpkg.Join(base, path)
		} else if !strings.HasPrefix(path, "public/") {
			return raw, nil
		}
		if !safeDeckRelPath(path) || !strings.HasPrefix(pathpkg.Clean(path), "public/") {
			return "", fmt.Errorf("snapshot asset escapes published public directory: %q", raw)
		}
		path = pathpkg.Clean(path)
		count := func(encoded string) (string, error) {
			encoded += snapshotFragment(parsed)
			expanded += len(encoded)
			if expanded > snapshotTotalLimit {
				return "", fmt.Errorf("snapshot expanded assets exceed the 96 MiB size budget")
			}
			return encoded, nil
		}
		if data, found := cache[path]; found {
			return count(data)
		}
		if pending[path] {
			return "", fmt.Errorf("snapshot CSS import cycle at %q", path)
		}
		file, err := directory.Open(filepath.FromSlash(path))
		if err != nil {
			return "", fmt.Errorf("snapshot asset %q: %w", raw, err)
		}
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			file.Close()
			return "", fmt.Errorf("snapshot asset %q is not a regular file", raw)
		}
		data, err := io.ReadAll(io.LimitReader(file, snapshotAssetLimit+1))
		file.Close()
		if err != nil {
			return "", err
		}
		total += len(data)
		if len(data) > snapshotAssetLimit || total > snapshotTotalLimit {
			return "", fmt.Errorf("snapshot local assets exceed size budget (16 MiB each, 96 MiB total)")
		}
		if strings.EqualFold(filepath.Ext(path), ".css") {
			if depth >= snapshotCSSDepthLimit {
				return "", fmt.Errorf("snapshot CSS imports exceed the %d level nesting limit", snapshotCSSDepthLimit)
			}
			pending[path] = true
			updated, err := css(string(data), pathpkg.Dir(path), depth+1)
			delete(pending, path)
			if err != nil {
				return "", err
			}
			if len(updated) > snapshotAssetLimit {
				return "", fmt.Errorf("snapshot expanded stylesheet %q exceeds the 16 MiB size budget", path)
			}
			data = []byte(updated)
		}
		kind := mime.TypeByExtension(filepath.Ext(path))
		if kind == "" {
			kind = "application/octet-stream"
		}
		if base64.StdEncoding.EncodedLen(len(data))+expanded > snapshotTotalLimit {
			return "", fmt.Errorf("snapshot expanded assets exceed the 96 MiB size budget")
		}
		encoded := "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data)
		cache[path] = encoded
		return count(encoded)
	}
	css = func(source, base string, depth int) (string, error) {
		return rewriteSnapshotCSS(source, func(value string) (string, error) {
			return inline(value, base, depth)
		})
	}
	var out bytes.Buffer
	tokenizer := html.NewTokenizer(strings.NewReader(document))
	inStyle := false
	for {
		typeOfToken := tokenizer.Next()
		if typeOfToken == html.ErrorToken {
			if tokenizer.Err() != io.EOF {
				return "", tokenizer.Err()
			}
			return out.String(), nil
		}
		raw := string(tokenizer.Raw())
		if typeOfToken == html.TextToken && inStyle {
			updated, err := css(raw, "", 0)
			if err != nil {
				return "", err
			}
			out.WriteString(updated)
			continue
		}
		if typeOfToken != html.StartTagToken && typeOfToken != html.SelfClosingTagToken && typeOfToken != html.EndTagToken {
			out.WriteString(raw)
			continue
		}
		token := tokenizer.Token()
		if token.Data == "style" {
			inStyle = typeOfToken != html.EndTagToken
		}
		if typeOfToken == html.EndTagToken {
			out.WriteString(raw)
			continue
		}
		changed := false
		for index, attr := range token.Attr {
			value := attr.Val
			switch {
			case attr.Key == "style":
				value, err = css(value, "", 0)
			case (token.Data == "img" || token.Data == "source" || token.Data == "video" || token.Data == "audio") && (attr.Key == "src" || attr.Key == "poster"):
				value, err = inline(value, "", 0)
			case token.Data == "image" && (attr.Key == "href" || attr.Key == "xlink:href"):
				value, err = inline(value, "", 0)
			case token.Data == "link" && attr.Key == "href" && snapshotStylesheetLink(token):
				value, err = inline(value, "", 0)
			case (token.Data == "img" || token.Data == "source") && attr.Key == "srcset" && !strings.Contains(value, "data:"):
				parts := strings.Split(value, ",")
				for i, part := range parts {
					fields := strings.Fields(part)
					if len(fields) == 0 {
						continue
					}
					fields[0], err = inline(fields[0], "", 0)
					if err != nil {
						break
					}
					parts[i] = strings.Join(fields, " ")
				}
				value = strings.Join(parts, ", ")
			}
			if err != nil {
				return "", err
			}
			if attr.Val != value {
				token.Attr[index].Val = value
				changed = true
			}
		}
		if changed {
			out.WriteString(token.String())
		} else {
			out.WriteString(raw)
		}
	}
}

// CSS references are recognized only outside comments and ordinary strings.
// A content:"url(...)" example or a disabled @import must stay literal, and
// must not cause files to be read just because they resemble resource syntax.
func rewriteSnapshotCSS(source string, inline func(string) (string, error)) (string, error) {
	var out strings.Builder
	for offset := 0; offset < len(source); {
		rest := source[offset:]
		if strings.HasPrefix(rest, "/*") {
			end := strings.Index(rest[2:], "*/")
			if end < 0 {
				out.WriteString(rest)
				break
			}
			end += 4
			out.WriteString(rest[:end])
			offset += end
			continue
		}
		var match []string
		imported := len(rest) >= 7 && rest[0] == '@' && strings.EqualFold(rest[:7], "@import")
		if imported {
			match = snapshotCSSImport.FindStringSubmatch(rest)
		} else if len(rest) >= 4 && (rest[0] == 'u' || rest[0] == 'U') && strings.EqualFold(rest[:4], "url(") && (offset == 0 || !snapshotCSSIdentifier(source[offset-1])) {
			match = snapshotCSSURLAtStart.FindStringSubmatch(rest)
		}
		if len(match) != 0 && strings.HasPrefix(rest, match[0]) {
			parts := match[1:]
			prefix := ""
			if imported {
				prefix = parts[0]
				parts = parts[1:]
			}
			value := ""
			for _, part := range parts {
				if part != "" {
					value = strings.TrimSpace(part)
					break
				}
			}
			replacement, err := inline(value)
			if err != nil {
				return "", err
			}
			if replacement == value {
				out.WriteString(match[0])
			} else {
				out.WriteString(prefix + `url("` + replacement + `")`)
			}
			offset += len(match[0])
			continue
		}
		if source[offset] == '\'' || source[offset] == '"' {
			quote, end := source[offset], offset+1
			for end < len(source) {
				if source[end] == '\\' {
					end = min(end+2, len(source))
				} else if source[end] == quote {
					end++
					break
				} else {
					end++
				}
			}
			out.WriteString(source[offset:end])
			offset = end
			continue
		}
		out.WriteByte(source[offset])
		offset++
	}
	return out.String(), nil
}

func snapshotCSSIdentifier(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' || character == '-' || character == '_' || character == '\\' || character >= 128
}

func tokenAttribute(token html.Token, key string) string {
	for _, attr := range token.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func snapshotStylesheetLink(token html.Token) bool {
	for _, word := range strings.Fields(tokenAttribute(token, "rel")) {
		if strings.EqualFold(word, "stylesheet") {
			return true
		}
	}
	return false
}

func snapshotFragment(parsed *url.URL) string {
	if parsed.Fragment != "" {
		return "#" + parsed.EscapedFragment()
	}
	return ""
}
