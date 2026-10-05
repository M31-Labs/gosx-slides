package slides

import (
	_ "embed"
	"m31labs.dev/gosx"
)

//go:embed assets/team.css
var teamStyle string

//go:embed assets/team.js
var teamScript string

func teamAssets() gosx.Node {
	return gosx.RawHTML("<style>" + teamStyle + "</style><script>" + teamScript + "</script>")
}
