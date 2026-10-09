package slides

import (
	"fmt"
	"html"
	"strings"
)

// Keep the authored frame intact, fitting it above an optional navigation strip.
// Real same-document anchors become PDF link annotations in Chrome's print path;
// there is no PDF JavaScript or dependency on a reader's transition support.
func pdfNavigationStyle(enabled bool) string {
	if !enabled {
		return ""
	}
	return `main.deck .capture-frame{height:calc(100vh - 36px)}
.capture-navigation{position:absolute;inset:auto 0 0;height:36px;box-sizing:border-box;display:flex;align-items:center;gap:20px;padding:0 20px;background:#111827;color:#f9fafb;font:12px/1.2 system-ui,sans-serif}
.capture-navigation a{color:#bae6fd;text-decoration:underline;white-space:nowrap}
.capture-navigation a:focus-visible{outline:2px solid #bae6fd;outline-offset:3px}
.capture-navigation span[aria-disabled]{color:#9ca3af;white-space:nowrap}
.capture-navigation .capture-state-label{flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;text-align:center}`
}

func pdfNavigation(page int, hasNext bool, label string) string {
	var out strings.Builder
	out.WriteString(`<nav class="capture-navigation" aria-label="Slide state navigation">`)
	if page > 0 {
		fmt.Fprintf(&out, `<a href="#capture-%d" aria-label="Previous state">Previous</a>`, page-1)
	} else {
		out.WriteString(`<span aria-disabled="true">Previous</span>`)
	}
	fmt.Fprintf(&out, `<span class="capture-state-label">%d · %s</span>`, page+1, html.EscapeString(label))
	if hasNext {
		fmt.Fprintf(&out, `<a href="#capture-%d" aria-label="Next state">Next</a>`, page+1)
	} else {
		out.WriteString(`<span aria-disabled="true">Next</span>`)
	}
	out.WriteString(`</nav>`)
	return out.String()
}
