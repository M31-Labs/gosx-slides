package slides

import _ "embed"

// Source assets stay readable and require no JavaScript build toolchain.
//
//go:embed assets/motion-timeline.js
var motionTimelineScript string

//go:embed assets/authoring.css
var authoringStyle string

//go:embed assets/lazy-islands.js
var lazyIslandScript string

//go:embed assets/morph.js
var morphScript string

//go:embed assets/readability.js
var readabilityScript string

//go:embed assets/code-morph.js
var codeMorphScript string
