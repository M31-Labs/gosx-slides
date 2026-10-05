package slides

import (
	"fmt"
	"strconv"
	"strings"
)

func officePageCSS(width, height int) string {
	return fmt.Sprintf("@page { size: %dpx %dpx; margin: 0; }", width, height)
}

// exportSize is both the capture viewport in CSS pixels and the physical PPTX
// slide size at 96 DPI. Explicit dimensions take precedence over aspect flags.
func exportSize(deck *IslandDeck, opts ExportOptions) (int, int, error) {
	if opts.Width != 0 || opts.Height != 0 {
		if opts.Width < 320 || opts.Height < 320 || opts.Width > 4096 || opts.Height > 4096 || int64(opts.Width)*int64(opts.Height) > 8_000_000 {
			return 0, 0, fmt.Errorf("export width and height must both be 320–4096 pixels, with at most 8 million pixels")
		}
		return opts.Width, opts.Height, nil
	}
	aspect := normalizedAspect(opts.Aspect)
	if aspect == "" && deck != nil {
		aspect = normalizedAspect(frontmatterText(deckFrontmatterValues(deck), "aspect-ratio"))
	}
	if aspect == "" || aspect == "16:9" {
		return 1280, 720, nil
	}
	if aspect == "4:3" {
		return 960, 720, nil
	}
	parts := strings.Split(aspect, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("aspect must be a ratio such as 16:9 or 4:3")
	}
	a, errA := strconv.ParseFloat(parts[0], 64)
	b, errB := strconv.ParseFloat(parts[1], 64)
	if errA != nil || errB != nil || !(a > 0 && b > 0 && a/b >= 0.5 && a/b <= 3) {
		return 0, 0, fmt.Errorf("aspect ratio must be finite and between 1:2 and 3:1")
	}
	return int(720*a/b + 0.5), 720, nil
}
