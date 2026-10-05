// Package mathassets contains the pinned, offline KaTeX distribution.
package mathassets

import "embed"

//go:embed katex.min.js katex.min.css LICENSE fonts/*.woff2
var Files embed.FS
