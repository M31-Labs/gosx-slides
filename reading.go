package slides

import _ "embed"

// Reading view uses the existing slide DOM and preserves interactive state.
//
//go:embed assets/reading.css
var readingStyle string

//go:embed assets/reading.js
var readingScript string
