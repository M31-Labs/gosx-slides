package slides

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestUndeclaredMotionCuePreservesDeclaredAddresses(t *testing.T) {
	for _, before := range []string{"", ":::motion {cue=request}\nRequest\n:::\n\n"} {
		t.Run(before, func(t *testing.T) {
			deck := graphicsDeck(t, "```yaml\nid: pipeline\ncues: overview, request, worker\n```\n\n# Pipeline\n\n"+before+":::motion {cue=extra}\nExtra\n:::\n", nil)
			names := slideCueNames(deck.Slides[0])
			if strings.Join(names, ",") != "overview,request,worker,extra" {
				t.Fatalf("declared cue addresses changed: %v", names)
			}
			if got := Analyze(deck).Slides[0].Clicks; got != 3 {
				t.Fatalf("extra cue click budget = %d, want 3", got)
			}
			data, err := json.Marshal(names)
			if err != nil {
				t.Fatal(err)
			}
			runNavigationJS(t, navLinkScript()+`
 const assert = require('node:assert/strict');
 const cueNames = `+string(data)+`;
 const slides = [{getAttribute:key=>key==='data-slide-id'?'pipeline':key==='data-slide-cues'?JSON.stringify(cueNames):null}];
 assert.deepEqual(readPosition('#pipeline/request',1),{index:0,step:1});
 assert.deepEqual(readPosition('#pipeline/worker',1),{index:0,step:2});
 assert.deepEqual(readPosition('#pipeline/extra',1),{index:0,step:3});
 assert.equal(positionHash(0,1,false),'#pipeline/request');
 assert.equal(positionHash(0,2,false),'#pipeline/worker');
 assert.equal(positionHash(0,3,false),'#pipeline/extra');
 `)
		})
	}
}

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
	deck := graphicsDeck(t, "```yaml\nid: code\ncues: overview, worker\nmorph-duration: 800\nmotion-duration: 1200\n```\n\n# Code\n\n:::motion {cue=worker step=3}\nWork\n:::\n\n:::code-morph\n```go\nold()\n```\n\n```go\nnew()\n```\n:::\n", nil)
	body := graphicsBody(t, deck)
	if !strings.Contains(body, `data-morph-duration="800"`) || !strings.Contains(body, `data-motion-duration="1200"`) || !strings.Contains(body, `slides-code-morph`) {
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
