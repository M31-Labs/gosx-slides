# Bundled math renderer

KaTeX **0.19.0** is vendored from the official npm distribution:
https://registry.npmjs.org/katex/-/katex-0.19.0.tgz

Archive SHA-256: `d8e49f2fea6eeed7cdae2cf4fd48e2f1d348758aa9e5740715e4533c2c96d1ee`.
The JavaScript, stylesheet, and WOFF2 fonts are unmodified distribution files.
The MIT license is retained in `LICENSE` and carried in generated stylesheets.

`math.go` executes the JavaScript server-side through Goja. Only the generated
HTML/MathML and stylesheet with embedded WOFF2 fonts reach a deck. There is no
browser math script, CDN, npm install, or Node requirement for deck authors.

When upgrading, replace the JavaScript, CSS, fonts and license from one pinned
upstream version, update this notice and the version markers in `math.go` and
`serve.go`, and run the Go math tests and `scripts/math-browser.cjs`.
