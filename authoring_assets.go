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

//go:embed assets/editing.js
var editingScript string

//go:embed assets/editing.css
var editingStyle string

//go:embed assets/diagram-motion.js
var diagramMotionScript string

//go:embed assets/pptx-editable.js
var pptxEditableScript string

//go:embed assets/graphics-motion.js
var graphicsMotionScript string

//go:embed assets/scene-studio.js
var sceneStudioScript string

//go:embed assets/background-wizard.js
var backgroundWizardScript string

//go:embed assets/background-wizard.css
var backgroundWizardStyle string
