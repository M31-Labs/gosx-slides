package slides

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVideoNarrationOptionsAndAssetContainment(t *testing.T) {
	for _, opts := range []ExportOptions{{Narration: "voice.wav"}, {Captions: "speech.vtt"}} {
		if err := validateVideoNarrationOptions(opts, "single"); err == nil {
			t.Fatal("narration accepted outside video export")
		}
	}
	for _, path := range []string{"../voice.wav", "/tmp/voice.wav", `\voice.wav`} {
		if err := validateVideoNarrationOptions(ExportOptions{Narration: path}, "video"); err == nil {
			t.Fatalf("escape accepted: %s", path)
		}
	}
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "speech.vtt")
	if err := os.WriteFile(outside, []byte("WEBVTT\n\n00:00.000 --> 00:01.000\nPrivate\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "speech.vtt")); err == nil {
		if n, err := prepareVideoNarration(context.Background(), &IslandDeck{Dir: dir}, ExportOptions{Captions: "speech.vtt"}); err == nil {
			n.close()
			t.Fatal("caption symlink outside deck was read")
		}
	}
}

func TestAuthoredVideoCaptionsAndVTTValidation(t *testing.T) {
	cues := []videoCaption{{0, 1000, "Don't infer <speech> & notes"}, {1000, 2000, ""}, {2000, 3000, "Explicit second line"}}
	vtt := authoredVideoVTT(cues)
	if !strings.Contains(string(vtt), "Don't infer &lt;speech&gt; &amp; notes") || strings.Contains(string(vtt), "&#39;") {
		t.Fatal("caption text was not valid WebVTT escaped text")
	}
	if until, err := validateVTT(vtt); err != nil || until != 3 {
		t.Fatalf("generated WebVTT failed: %f %v", until, err)
	}
	if authoredVideoVTT([]videoCaption{{0, 1000, ""}}) != nil {
		t.Fatal("a caption was inferred from empty metadata")
	}
	for _, text := range []string{"no header", "WEBVTT\n\n00:00.000 --> 00:00.000\nEmpty duration", "WEBVTT\n\n00:61.000 --> 00:62.000\nInvalid seconds", "WEBVTT\n\n9999999999999999999999:00:00.000 --> 9999999999999999999999:00:01.000\nOverflow", "WEBVTT\n\n00:02.000 --> 00:03.000\nLater\n\n00:01.000 --> 00:02.000\nEarlier", "WEBVTT\n\n00:00.000 --> 00:01.000"} {
		if _, err := validateVTT([]byte(text)); err == nil {
			t.Errorf("invalid captions accepted: %q", text)
		}
	}
	if n := (&videoNarration{audioDuration: 2}); n.finish(context.Background(), "unused", filepath.Join(t.TempDir(), "out.webm"), 1, nil) == nil {
		t.Fatal("long narration was silently truncated")
	}
	if n := (&videoNarration{captionEnd: 2}); n.finish(context.Background(), "unused", filepath.Join(t.TempDir(), "out.webm"), 1, nil) == nil {
		t.Fatal("out-of-video caption timings were accepted")
	}
}

func TestNarrationMuxPadsShortAudioAndKeepsAuthoredVTT(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := t.TempDir()
	audio := filepath.Join(dir, "voice.wav")
	if logs, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.2", "-y", audio).CombinedOutput(); err != nil {
		t.Fatalf("make audio: %v %s", err, logs)
	}
	vtt := []byte("WEBVTT\n\n00:00.000 --> 00:00.500\nAuthored narration\n")
	if err := os.WriteFile(filepath.Join(dir, "voice.vtt"), vtt, 0600); err != nil {
		t.Fatal(err)
	}
	n, err := prepareVideoNarration(context.Background(), &IslandDeck{Dir: dir}, ExportOptions{Narration: "voice.wav", Captions: "voice.vtt"})
	if err != nil {
		t.Fatal(err)
	}
	defer n.close()
	if logs, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=c=blue:s=160x90:d=0.5", "-c:v", "libvpx-vp9", "-y", n.video).CombinedOutput(); err != nil {
		t.Fatalf("make video: %v %s", err, logs)
	}
	out := filepath.Join(dir, "talk.webm")
	if err := n.finish(context.Background(), ffmpeg, out, 0.5, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "talk.vtt"))
	if err != nil || string(data) != string(vtt) {
		t.Fatalf("authored sidecar not preserved: %v", err)
	}
	logs, err := exec.Command(probe, "-v", "error", "-show_entries", "stream=codec_type:format=duration", "-of", "json", out).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(logs, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Streams) != 2 || report.Streams[0].CodecType != "video" || report.Streams[1].CodecType != "audio" {
		t.Fatalf("mux did not retain video/audio: %s", logs)
	}
}

func TestVideoExportNarrationAndCaptionMetadata(t *testing.T) {
	if _, err := findChrome(); err != nil {
		t.Skip("Chrome unavailable")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg unavailable")
	}
	probe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe unavailable")
	}
	dir := newDeckDirUnderModule(t, "---\ntitle: Narrated video\noffline-required: true\n---\n\n```yaml\ncaption: Explicitly authored export caption\n```\n\n# Recorded state\n", nil)
	if logs, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=0.2", "-y", filepath.Join(dir, "voice.wav")).CombinedOutput(); err != nil {
		t.Fatalf("make narration: %v %s", err, logs)
	}
	out := filepath.Join(t.TempDir(), "talk.webm")
	if err := ExportStatic(dir, ExportOptions{Format: "video", OutDir: out, Seconds: 0.5, FPS: 5, Narration: "voice.wav"}); err != nil {
		t.Fatal(err)
	}
	vtt, err := os.ReadFile(strings.TrimSuffix(out, ".webm") + ".vtt")
	if err != nil || !strings.Contains(string(vtt), "Explicitly authored export caption") {
		t.Fatalf("export did not write authored captions: %v %s", err, vtt)
	}
	if until, err := validateVTT(vtt); err != nil || until != 0.6 {
		t.Fatalf("captions do not follow the three encoded frames: %f %v", until, err)
	}
	logs, err := exec.Command(probe, "-v", "error", "-select_streams", "a:0", "-show_entries", "stream=codec_type", "-of", "csv=p=0", out).CombinedOutput()
	if err != nil || strings.TrimSpace(string(logs)) != "audio" {
		t.Fatalf("video export lost its narration: %v %s", err, logs)
	}
}
