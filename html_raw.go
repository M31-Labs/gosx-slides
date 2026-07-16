package slides

// html_raw.go is the sanitized raw-HTML passthrough lane. Ordinary HTML
// written in deck.md — block-level (<div class="grid">…</div>, <style>…) and
// inline (<br>, <span class="x">) — renders instead of being dropped, which is
// what makes free-form composition (columns, badges, designed line breaks,
// per-slide style blocks) possible without an island or a theme fork.
//
// Safety model: deck authors are trusted (they already ship arbitrary .gsx
// code), but PASTED or GENERATED markdown is not. The sanitizer guarantees
// nothing executable survives the passthrough: script-capable subtrees are
// removed (tag AND content), event-handler attributes are stripped, and URL
// attributes are scheme-checked. The generated GoSX source carries the raw
// literal as a quoted Go string (it can never corrupt the program); the
// sanitizer runs at render time inside the bound __slidesHTML.Raw call, the
// same RawHTML-Node-through-the-evaluator mechanism the code-block lane proved
// safe (see codeBlockNode).

import (
	"strings"

	"golang.org/x/net/html"

	"m31labs.dev/gosx"
)

// htmlNamespace / htmlRawFunc name the bound expression function slidegen emits
// for a raw HTML literal (e.g. `{__slidesHTML.Raw("<div class=\"x\">")}`).
// Consts so the generator and the render_program.go binding can never drift;
// the `__` prefix keeps the namespace out of any deck author's reach.
const (
	htmlNamespace = "__slidesHTML"
	htmlRawFunc   = "Raw"
)

// rawHTMLNode is the implementation behind {__slidesHTML.Raw(...)}: sanitize
// the fragment and return it as a RawHTML Node so the markup rides the
// expression evaluator unescaped (a plain string would be entity-escaped by
// the renderer and show up as visible angle brackets).
func rawHTMLNode(fragment string) gosx.Node {
	return gosx.RawHTML(sanitizeDeckHTML(fragment))
}

// rawHTMLAllowedTags is the passthrough element allowlist: structural and
// phrasing content plus media. Anything not here is dropped (children kept —
// drop the wrapper, keep the words). Notably ABSENT: script, iframe, object,
// embed, form and friends — see rawHTMLDroppedSubtrees for the ones whose
// CONTENT is dropped too.
var rawHTMLAllowedTags = map[string]bool{
	"a": true, "abbr": true, "article": true, "aside": true, "audio": true,
	"b": true, "blockquote": true, "br": true, "caption": true, "cite": true,
	"code": true, "col": true, "colgroup": true, "dd": true, "del": true,
	"details": true, "dfn": true, "div": true, "dl": true, "dt": true,
	"em": true, "figcaption": true, "figure": true, "footer": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"header": true, "hr": true, "i": true, "img": true, "ins": true,
	"kbd": true, "li": true, "main": true, "mark": true, "nav": true,
	"ol": true, "p": true, "picture": true, "pre": true, "q": true, "s": true,
	"samp": true, "section": true, "small": true, "source": true, "span": true,
	"strong": true, "style": true, "sub": true, "summary": true, "sup": true,
	"table": true, "tbody": true, "td": true, "tfoot": true, "th": true,
	"thead": true, "time": true, "tr": true, "track": true, "u": true,
	"ul": true, "var": true, "video": true, "wbr": true,
}

// rawHTMLVoidTags are elements with no close tag; they are re-emitted
// self-closed so the output is well-formed regardless of how the author wrote
// them.
var rawHTMLVoidTags = map[string]bool{
	"br": true, "col": true, "hr": true, "img": true, "source": true,
	"track": true, "wbr": true,
}

// rawHTMLDroppedSubtrees are elements removed together with their entire
// content — emitting a <script>'s text as prose would be as wrong as emitting
// the tag. (Style is NOT here: it is an allowed raw-text element.)
var rawHTMLDroppedSubtrees = map[string]bool{
	"script": true, "iframe": true, "object": true, "embed": true,
	"applet": true, "form": true, "input": true, "button": true,
	"select": true, "textarea": true, "link": true, "meta": true,
	"base": true, "noscript": true, "frame": true, "frameset": true,
	"title": true, "head": true, "svg": true, "math": true,
}

// rawHTMLGlobalAttrs are attributes allowed on every passthrough element.
// data-* and aria-* prefixes are additionally allowed (checked in
// sanitizeAttr). Event handlers (on*) are always stripped.
var rawHTMLGlobalAttrs = map[string]bool{
	"class": true, "id": true, "style": true, "title": true, "role": true,
	"dir": true, "lang": true, "hidden": true, "translate": true,
}

// rawHTMLTagAttrs are per-element attribute allowances beyond the globals.
// URL-carrying attributes (href, src, poster) are scheme-checked separately.
var rawHTMLTagAttrs = map[string]map[string]bool{
	"a":       {"href": true, "target": true, "rel": true, "download": true},
	"img":     {"src": true, "alt": true, "width": true, "height": true, "loading": true, "decoding": true},
	"video":   {"src": true, "poster": true, "controls": true, "autoplay": true, "muted": true, "loop": true, "playsinline": true, "width": true, "height": true, "preload": true},
	"audio":   {"src": true, "controls": true, "autoplay": true, "muted": true, "loop": true, "preload": true},
	"source":  {"src": true, "type": true, "media": true, "srcset": true},
	"track":   {"src": true, "kind": true, "srclang": true, "label": true, "default": true},
	"td":      {"colspan": true, "rowspan": true},
	"th":      {"colspan": true, "rowspan": true, "scope": true},
	"col":     {"span": true},
	"colgroup": {"span": true},
	"time":    {"datetime": true},
	"details": {"open": true},
	"ol":      {"start": true, "reversed": true, "type": true},
	"q":       {"cite": true},
	"blockquote": {"cite": true},
	"del":     {"cite": true, "datetime": true},
	"ins":     {"cite": true, "datetime": true},
}

// rawHTMLURLAttrs are the attributes whose values are URLs and must pass
// safeDeckURL. A failing value drops the attribute, never the element.
var rawHTMLURLAttrs = map[string]bool{
	"href": true, "src": true, "poster": true, "cite": true, "srcset": true,
}

// safeDeckURL reports whether a URL attribute value is safe to pass through:
// http(s), mailto, tel, fragment, or relative URLs; data: is allowed for
// images only. Everything else — javascript:, vbscript:, data:text/html — is
// rejected.
func safeDeckURL(tag, val string) bool {
	v := strings.TrimSpace(strings.ToLower(val))
	switch {
	case v == "" || strings.HasPrefix(v, "#") || strings.HasPrefix(v, "/") ||
		strings.HasPrefix(v, "./") || strings.HasPrefix(v, "../"):
		return true
	case strings.HasPrefix(v, "http://"), strings.HasPrefix(v, "https://"),
		strings.HasPrefix(v, "mailto:"), strings.HasPrefix(v, "tel:"):
		return true
	case strings.HasPrefix(v, "data:image/"):
		return tag == "img" || tag == "source"
	case strings.Contains(v, ":"):
		// Any other explicit scheme (javascript:, vbscript:, data:text/…) is out.
		return false
	default:
		// Scheme-less relative path (e.g. "img/x.png").
		return true
	}
}

// sanitizeAttr reports whether one attribute survives on the given element,
// returning the (unchanged) value. Order of checks: event handlers out first,
// then data-/aria- prefixes, then the global and per-tag allowlists, with URL
// attributes additionally scheme-checked.
func sanitizeAttr(tag, key, val string) bool {
	k := strings.ToLower(key)
	if strings.HasPrefix(k, "on") {
		return false
	}
	if rawHTMLURLAttrs[k] && !safeDeckURL(tag, val) {
		return false
	}
	if strings.HasPrefix(k, "data-") || strings.HasPrefix(k, "aria-") {
		return true
	}
	if rawHTMLGlobalAttrs[k] {
		return true
	}
	if allowed, ok := rawHTMLTagAttrs[tag]; ok && allowed[k] {
		return true
	}
	return false
}

// sanitizeDeckHTML rewrites an HTML fragment through the x/net/html tokenizer,
// emitting only allowlisted elements and attributes. Text is re-escaped on
// output (except inside <style>, a raw-text element whose CSS must keep its
// `>` combinators); comments and doctypes vanish; dropped-subtree elements
// take their content with them. Unbalanced fragments are fine — a stray close
// tag for an allowed element is emitted as-is (mdpp splits paired block tags
// into separate literals, so open and close arrive in different calls).
func sanitizeDeckHTML(fragment string) string {
	z := html.NewTokenizer(strings.NewReader(fragment))
	var b strings.Builder
	skipUntil := "" // inside a dropped subtree: the tag whose close ends the skip
	skipDepth := 0
	inStyle := false

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return b.String() // io.EOF or a tokenize error: emit what we have
		}
		switch tt {
		case html.TextToken:
			if skipUntil != "" {
				continue
			}
			if inStyle {
				// Raw-text element: entities are not parsed in <style>, so the
				// content must pass through byte-for-byte (escaping would corrupt
				// selectors like `a > b`). The tokenizer guarantees this text
				// cannot contain "</style", so it cannot break out of the element.
				b.Write(z.Raw())
				continue
			}
			b.WriteString(html.EscapeString(string(z.Text())))

		case html.StartTagToken, html.SelfClosingTagToken:
			tok := z.Token()
			name := strings.ToLower(tok.Data)
			if skipUntil != "" {
				if name == skipUntil {
					skipDepth++
				}
				continue
			}
			if rawHTMLDroppedSubtrees[name] {
				if tt != html.SelfClosingTagToken && !rawHTMLVoidTags[name] {
					skipUntil, skipDepth = name, 1
				}
				continue
			}
			if !rawHTMLAllowedTags[name] {
				continue // drop the wrapper, keep the children
			}
			if name == "style" && tt != html.SelfClosingTagToken {
				inStyle = true
			}
			b.WriteByte('<')
			b.WriteString(name)
			for _, attr := range tok.Attr {
				if !sanitizeAttr(name, attr.Key, attr.Val) {
					continue
				}
				b.WriteByte(' ')
				b.WriteString(strings.ToLower(attr.Key))
				b.WriteString(`="`)
				b.WriteString(html.EscapeString(attr.Val))
				b.WriteByte('"')
			}
			if rawHTMLVoidTags[name] {
				b.WriteString("/>")
			} else {
				b.WriteByte('>')
				if tt == html.SelfClosingTagToken {
					// Author self-closed a non-void element (<div/>): emit the
					// close immediately so the output stays balanced.
					b.WriteString("</" + name + ">")
					if name == "style" {
						inStyle = false
					}
				}
			}

		case html.EndTagToken:
			tok := z.Token()
			name := strings.ToLower(tok.Data)
			if skipUntil != "" {
				if name == skipUntil {
					skipDepth--
					if skipDepth == 0 {
						skipUntil = ""
					}
				}
				continue
			}
			if name == "style" {
				inStyle = false
			}
			if !rawHTMLAllowedTags[name] || rawHTMLVoidTags[name] {
				continue
			}
			b.WriteString("</" + name + ">")

		case html.CommentToken, html.DoctypeToken:
			// Dropped: comments are author notes (and the speaker-note form is
			// consumed upstream); doctypes have no place mid-document.
		}
	}
}
