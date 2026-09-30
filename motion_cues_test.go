package slides

import (
	"strings"
	"testing"
)

func TestNamedMotionCuesShareClickBudgetAndSurviveLowering(t *testing.T) {
	deck := graphicsDeck(t, "```yaml\nid: pipeline\ncues: overview, request, worker\n```\n\n# Pipeline\n\n:::motion {cue=request duration=300}\nRequest\n:::\n\n:::motion {cue=worker after=request}\nWorker\n:::\n\n:::motion {cue=complete step=4}\nComplete\n:::\n", nil)
	body := graphicsBody(t, deck)
	for _, want := range []string{`data-slide-id="pipeline"`, `data-slide-cues="[`, `data-slides-motion-cue="worker"`, `data-slides-motion-after="request"`, `data-slides-motion-step="4"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s", want)
		}
	}
	analysis := Analyze(deck)
	if analysis.Slides[0].Clicks != 4 {
		t.Fatalf("authoring budget = %d", analysis.Slides[0].Clicks)
	}
}
func TestNamedCueAddresses(t *testing.T) {
	runNavigationJS(t, navLinkScript()+`
 const assert = require('node:assert/strict');
 const slides = [{getAttribute:key=>key==='data-slide-id'?'pipeline':key==='data-slide-cues'?'["overview","request","worker"]':null}];
 assert.deepEqual(readPosition('#pipeline/worker',1),{index:0,step:2});
 assert.deepEqual(readPosition('#pipeline/2',1),{index:0,step:2});
 assert.deepEqual(readPosition('#pipeline',1),{index:0,step:0});
 assert.deepEqual(readPosition('#missing/worker',1),{index:0,step:0});
 assert.equal(positionHash(0,2,false),'#pipeline/worker');
 assert.equal(positionHash(0,2,true),'#1/2present');
 `)
}

func TestCodeMorphClickBudgetAndMovedCue(t *testing.T) {
	deck := graphicsDeck(t, "```yaml\nid: code\ncues: overview, worker\nmorph-duration: 800\n```\n\n# Code\n\n:::motion {cue=worker step=3}\nWork\n:::\n\n:::code-morph\n```go\nold()\n```\n\n```go\nnew()\n```\n:::\n", nil)
	body := graphicsBody(t, deck)
	if !strings.Contains(body, `data-morph-duration="800"`) || !strings.Contains(body, `slides-code-morph`) {
		t.Fatal("missing authored morph controls")
	}
	names := slideCueNames(deck.Slides[0])
	if names[1] != "" || names[3] != "worker" {
		t.Fatalf("ambiguous cue address: %v", names)
	}
	if Analyze(deck).Slides[0].Clicks != 3 {
		t.Fatal("lost click budget")
	}
}
