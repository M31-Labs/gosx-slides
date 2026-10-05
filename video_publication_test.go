package slides

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func priorVideoFiles(t *testing.T, dir string) (string, string) {
	t.Helper()
	video, caption := filepath.Join(dir, "talk.webm"), filepath.Join(dir, "talk.vtt")
	if err := os.WriteFile(video, []byte("previous complete video"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caption, []byte("previous complete captions"), 0600); err != nil {
		t.Fatal(err)
	}
	return video, caption
}
func assertPriorVideoFiles(t *testing.T, video, caption string) {
	t.Helper()
	for filename, want := range map[string]string{video: "previous complete video", caption: "previous complete captions"} {
		data, err := os.ReadFile(filename)
		if err != nil || string(data) != want {
			t.Fatalf("prior output changed: %s %v %q", filename, err, data)
		}
	}
}

func TestVideoPublicationRollbackAndPermissions(t *testing.T) {
	dir := t.TempDir()
	video, caption := priorVideoFiles(t, dir)
	stage := filepath.Join(dir, "stage")
	os.Mkdir(stage, 0700)
	v, c := filepath.Join(stage, "new.webm"), filepath.Join(stage, "new.vtt")
	os.WriteFile(v, []byte("new video"), 0644)
	os.WriteFile(c, []byte("new captions"), 0644)
	rename := func(source, destination string) error {
		if source == c && destination == caption {
			return errors.New("injected second publication failure")
		}
		return os.Rename(source, destination)
	}
	if err := publishVideoFiles(stage, []videoPublication{{staged: v, target: video}, {staged: c, target: caption}}, rename); err == nil {
		t.Fatal("failed publication accepted")
	}
	assertPriorVideoFiles(t, video, caption)
	for filename, want := range map[string]string{v: "new video", c: "new captions"} {
		data, err := os.ReadFile(filename)
		if err != nil || string(data) != want {
			t.Fatal("completed replacement lost during rollback")
		}
	}
	// Use fresh staged files after rollback, then verify a successful transaction.
	os.WriteFile(v, []byte("new video"), 0644)
	if err := publishVideoFiles(stage, []videoPublication{{staged: v, target: video}, {staged: c, target: caption}}, os.Rename); err != nil {
		t.Fatal(err)
	}
	for filename, want := range map[string]string{video: "new video", caption: "new captions"} {
		data, _ := os.ReadFile(filename)
		if string(data) != want {
			t.Fatal("publication content mismatch")
		}
		if runtime.GOOS != "windows" {
			info, _ := os.Stat(filename)
			if info.Mode().Perm() != 0600 {
				t.Fatal("private output permissions were lost")
			}
		}
	}
}

func TestNarrationValidationAndPublicationFailuresPreserveOutputs(t *testing.T) {
	for _, test := range []struct {
		name           string
		audio, caption float64
	}{{"audio", 2, 0}, {"captions", 0, 2}} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			video, caption := priorVideoFiles(t, dir)
			n := &videoNarration{audioDuration: test.audio, captionEnd: test.caption}
			defer n.close()
			if err := n.finish(context.Background(), "unused", video, 1, nil); err == nil {
				t.Fatal("invalid timing accepted")
			}
			assertPriorVideoFiles(t, video, caption)
		})
	}
	dir := t.TempDir()
	video, caption := priorVideoFiles(t, dir)
	n := &videoNarration{}
	defer n.close()
	if err := n.stageVideo(video); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(n.video, []byte("staged complete video"), 0644)
	n.captions = []byte("WEBVTT\n\n00:00.000 --> 00:01.000\nNew\n")
	// A sidecar directory must be rejected before the prior video is replaced.
	os.Remove(caption)
	os.Mkdir(caption, 0700)
	if err := n.finish(context.Background(), "unused", video, 1, nil); err == nil {
		t.Fatal("sidecar directory overwritten")
	}
	data, _ := os.ReadFile(video)
	if string(data) != "previous complete video" {
		t.Fatal("video replaced before sidecar preflight")
	}
	if info, _ := os.Stat(caption); !info.IsDir() {
		t.Fatal("sidecar directory changed")
	}
	if !n.keepOutputStage {
		t.Fatal("publication recovery staging was deleted")
	}
	conflict := filepath.Join(t.TempDir(), "talk.VTT")
	os.WriteFile(conflict, []byte("prior"), 0644)
	if err := n.finish(context.Background(), "unused", conflict, 1, nil); err == nil {
		t.Fatal("video/caption alias accepted")
	}
	data, _ = os.ReadFile(conflict)
	if string(data) != "prior" {
		t.Fatal("aliased destination overwritten")
	}
}

// The helper intentionally creates partial output before failing, exercising
// preservation after a late process error rather than an input-open error.
func failingVideoEncoder(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX late-process-failure helper; portable rollback is tested separately")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "ffmpeg")
	body := "#!/bin/sh\nfor arg do target=\"$arg\"; done\ncat >/dev/null\nprintf partial >\"$target\"\nexit 1\n"
	if err := os.WriteFile(exe, []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	return exe
}

func TestNarrationLateMuxFailurePreservesVideoAndVTT(t *testing.T) {
	ffmpeg := failingVideoEncoder(t)
	dir := t.TempDir()
	video, caption := priorVideoFiles(t, dir)
	n := &videoNarration{audio: "unused.wav", audioDuration: .5, captions: []byte("authored captions")}
	defer n.close()
	if err := n.stageVideo(video); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(n.video, []byte("complete silent video"), 0644)
	if err := n.finish(context.Background(), ffmpeg, video, 1, nil); err == nil {
		t.Fatal("failing mux accepted")
	}
	assertPriorVideoFiles(t, video, caption)
}

func TestVideoEncodeAndCaptionValidationFailuresPreserveOutputs(t *testing.T) {
	if os.Getenv("SLIDES_CHROME") == "" {
		t.Skip("set SLIDES_CHROME for browser capture regression")
	}
	for _, kind := range []string{"caption-validation", "late-encoder"} {
		t.Run(kind, func(t *testing.T) {
			dir := newDeckDirUnderModule(t, "---\noffline-required: true\n---\n\n# Protected outputs\n", nil)
			os.WriteFile(filepath.Join(dir, "long.vtt"), []byte("WEBVTT\n\n00:00.000 --> 00:04.000\nToo long\n"), 0644)
			video, caption := priorVideoFiles(t, t.TempDir())
			opts := ExportOptions{Format: "video", OutDir: video, Seconds: .1, FPS: 5}
			if kind == "caption-validation" {
				opts.Captions = "long.vtt"
			} else {
				helper := failingVideoEncoder(t)
				t.Setenv("PATH", filepath.Dir(helper)+string(os.PathListSeparator)+os.Getenv("PATH"))
			}
			if err := ExportStatic(dir, opts); err == nil {
				t.Fatal("failed export accepted")
			}
			assertPriorVideoFiles(t, video, caption)
			if entries, _ := filepath.Glob(filepath.Join(filepath.Dir(video), ".slides-video-*")); len(entries) != 0 {
				t.Fatalf("failed export left temporary staging: %v", entries)
			}
		})
	}
}

func TestSilentVideoPublicationSuccess(t *testing.T) {
	dir := t.TempDir()
	video, caption := priorVideoFiles(t, dir)
	n := &videoNarration{}
	defer n.close()
	if err := n.stageVideo(video); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(n.video, []byte("new complete video"), 0644)
	if err := n.finish(context.Background(), "unused", video, 1, []videoCaption{{0, 1000, "New caption"}}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(video)
	if string(data) != "new complete video" {
		t.Fatal("silent video not published")
	}
	data, _ = os.ReadFile(caption)
	if !strings.Contains(string(data), "New caption") {
		t.Fatal("authored sidecar not published")
	}
}
