package slides

import "strings"

// BuiltInThemes returns the stable names accepted by the local renderer.
func BuiltInThemes() []string {
	return []string{"m31", "noir", "blueprint", "ember"}
}

func isKnownTheme(theme string) bool {
	theme = themeClass(theme)
	for _, known := range BuiltInThemes() {
		if theme == known {
			return true
		}
	}
	return false
}

func themeClass(theme string) string {
	theme = strings.TrimSpace(strings.ToLower(theme))
	if theme == "" {
		return "m31"
	}
	return safeClass(theme)
}
