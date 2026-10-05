package slides

import (
	"encoding/json"
	"strings"
	"testing"

	"m31labs.dev/gosx"
)

func TestRecordingMetadataUsesExplicitCaptions(t *testing.T) {
	deck := loadDeckFromSource(t, "# Private notes are separate\n\n<!-- Private speaker note -->\n\n---\n\n```yaml\nid: finish\ncaption: Explicit caption with </script> content\n```\n\n# A title is not a transcript\n", nil)
	markup := gosx.RenderHTML(recordingMetadata(deck))
	if strings.Contains(markup, "Private speaker note") || strings.Contains(markup, "A title is not a transcript") {
		t.Fatal("recording metadata inferred captions or exposed speaker notes")
	}
	if strings.Count(markup, "</script>") != 1 {
		t.Fatal("caption escaped its JSON script element")
	}
	encoded := strings.TrimSuffix(strings.SplitN(markup, ">", 2)[1], "</script>")
	var slides []recordingSlide
	if err := json.Unmarshal([]byte(encoded), &slides); err != nil {
		t.Fatal(err)
	}
	if len(slides) != 2 || slides[0].Caption != "" || slides[1].Caption != "Explicit caption with </script> content" || slides[1].ID != "finish" {
		t.Fatalf("wrong metadata: %+v", slides)
	}
}
