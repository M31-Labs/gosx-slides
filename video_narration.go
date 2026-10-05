package slides

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const narrationAudioLimit = 256 << 20
const narrationCaptionLimit = 1 << 20
const narrationFormats = "wav,mp3,aac,flac,ogg,mov,matroska,webm"

type videoNarration struct {
	dir           string
	audio         string
	audioDuration float64
	captions      []byte
	captionEnd    float64
	video         string
}

func validateVideoNarrationOptions(opts ExportOptions, format string) error {
	if (opts.Narration != "" || opts.Captions != "") && format != "video" {
		return fmt.Errorf("narration and captions require --format video")
	}
	for _, name := range []string{opts.Narration, opts.Captions} {
		if name != "" && (!safeDeckRelPath(name) || filepath.IsAbs(name)) {
			return fmt.Errorf("narration/caption path must be relative to and inside the deck: %q", name)
		}
	}
	return nil
}

// Stage bounded copies via os.Root. ffmpeg never follows author symlinks or
// reads a URL, playlist, or path outside the deck. Its demuxers/protocols are
// explicitly restricted when probing and muxing the narration copy.
func prepareVideoNarration(ctx context.Context, deck *IslandDeck, opts ExportOptions) (*videoNarration, error) {
	n := &videoNarration{}
	if opts.Narration == "" && opts.Captions == "" {
		return n, nil
	}
	root, err := os.OpenRoot(deck.Dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	n.dir, err = os.MkdirTemp("", "slides-narration-*")
	if err != nil {
		return nil, err
	}
	failed := func(err error) (*videoNarration, error) { n.close(); return nil, err }
	read := func(name string, limit int64) ([]byte, error) {
		if !safeDeckRelPath(name) {
			return nil, fmt.Errorf("narration asset escapes deck: %q", name)
		}
		file, err := root.Open(filepath.FromSlash(name))
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("narration asset %q is not a regular file", name)
		}
		data, err := io.ReadAll(io.LimitReader(file, limit+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("narration asset %q exceeds its size limit", name)
		}
		return data, nil
	}
	if opts.Captions != "" {
		if !strings.EqualFold(filepath.Ext(opts.Captions), ".vtt") {
			return failed(fmt.Errorf("caption file must have a .vtt extension"))
		}
		n.captions, err = read(opts.Captions, narrationCaptionLimit)
		if err != nil {
			return failed(err)
		}
		n.captionEnd, err = validateVTT(n.captions)
		if err != nil {
			return failed(err)
		}
	}
	if opts.Narration != "" {
		ext := strings.ToLower(filepath.Ext(opts.Narration))
		switch ext {
		case ".wav", ".mp3", ".aac", ".flac", ".ogg", ".opus", ".m4a", ".webm":
		default:
			return failed(fmt.Errorf("narration must be WAV, MP3, AAC, FLAC, Ogg/Opus, M4A, or WebM audio"))
		}
		data, err := read(opts.Narration, narrationAudioLimit)
		if err != nil {
			return failed(err)
		}
		n.audio = filepath.Join(n.dir, "audio"+ext)
		if err = os.WriteFile(n.audio, data, 0600); err != nil {
			return failed(err)
		}
		probe, err := exec.LookPath("ffprobe")
		if err != nil {
			return failed(fmt.Errorf("audio narration needs ffprobe on PATH"))
		}
		probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		output, err := exec.CommandContext(probeCtx, probe, "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", narrationFormats, "-select_streams", "a:0", "-show_entries", "format=duration:stream=codec_type", "-of", "json", n.audio).CombinedOutput()
		if err != nil {
			return failed(fmt.Errorf("probe narration: %w: %.1000s", err, output))
		}
		var report struct {
			Format struct {
				Duration string `json:"duration"`
			} `json:"format"`
			Streams []struct {
				CodecType string `json:"codec_type"`
			} `json:"streams"`
		}
		if err = json.Unmarshal(output, &report); err != nil {
			return failed(err)
		}
		n.audioDuration, err = strconv.ParseFloat(report.Format.Duration, 64)
		if err != nil || !isFiniteDuration(n.audioDuration) || n.audioDuration > 3600 || len(report.Streams) != 1 || report.Streams[0].CodecType != "audio" {
			return failed(fmt.Errorf("narration needs one readable audio stream with a known duration between 0 and 3600 seconds"))
		}
		n.video = filepath.Join(n.dir, "silent.webm")
	}
	return n, nil
}

func isFiniteDuration(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
func (n *videoNarration) close() {
	if n != nil && n.dir != "" {
		os.RemoveAll(n.dir)
	}
}

type videoCaption struct {
	StartMS, EndMS int
	Text           string
}

func (n *videoNarration) finish(ctx context.Context, ffmpeg, output string, duration float64, cues []videoCaption) error {
	if n.audioDuration > duration+0.1 {
		return fmt.Errorf("narration is %.3fs but video is %.3fs; increase --seconds or shorten the audio", n.audioDuration, duration)
	}
	if n.captionEnd > duration+0.001 {
		return fmt.Errorf("caption ends at %.3fs but video is %.3fs; adjust the VTT timings or increase --seconds", n.captionEnd, duration)
	}
	if n.audio != "" {
		command := exec.CommandContext(ctx, ffmpeg, "-y", "-hide_banner", "-loglevel", "error", "-i", n.video, "-protocol_whitelist", "file,pipe", "-format_whitelist", narrationFormats, "-i", n.audio, "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "libopus", "-af", "apad", "-t", strconv.FormatFloat(duration, 'f', 6, 64), "-f", "webm", output)
		if logs, err := command.CombinedOutput(); err != nil {
			return fmt.Errorf("mux narration: %w: %.1000s", err, logs)
		}
	}
	captionData := n.captions
	if len(captionData) == 0 {
		captionData = authoredVideoVTT(cues)
	}
	if len(captionData) != 0 {
		path := strings.TrimSuffix(output, filepath.Ext(output)) + ".vtt"
		if err := os.WriteFile(path, captionData, 0644); err != nil {
			return fmt.Errorf("write captions: %w", err)
		}
	}
	return nil
}

func authoredVideoVTT(cues []videoCaption) []byte {
	var body strings.Builder
	count := 0
	for _, cue := range cues {
		if cue.Text == "" || cue.EndMS <= cue.StartMS {
			continue
		}
		count++
		text := strings.Join(strings.Fields(cue.Text), " ")
		text = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(text)
		fmt.Fprintf(&body, "%d\n%s --> %s\n%s\n\n", count, vttTimestamp(cue.StartMS), vttTimestamp(cue.EndMS), text)
	}
	if count == 0 {
		return nil
	}
	return []byte("WEBVTT\n\nNOTE Explicit authored caption metadata, timed to exported slide states.\n\n" + body.String())
}

func vttTimestamp(ms int) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

var vttTiming = regexp.MustCompile(`^((?:[0-9]{2,}:)?[0-9]{2}:[0-9]{2}\.[0-9]{3})[ \t]+-->[ \t]+((?:[0-9]{2,}:)?[0-9]{2}:[0-9]{2}\.[0-9]{3})(?:[ \t]+.*)?$`)

func validateVTT(data []byte) (float64, error) {
	if !utf8.Valid(data) || len(data) > narrationCaptionLimit || strings.ContainsRune(string(data), 0) {
		return 0, fmt.Errorf("captions must be UTF-8 WebVTT up to 1 MiB")
	}
	text := strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\ufeff")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || !(lines[0] == "WEBVTT" || strings.HasPrefix(lines[0], "WEBVTT ")) {
		return 0, fmt.Errorf("caption file must start with WEBVTT")
	}
	blocks := strings.Split(strings.TrimSpace(strings.Join(lines[1:], "\n")), "\n\n")
	end, previous, count := 0.0, -1.0, 0
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "NOTE ") || strings.HasPrefix(block, "NOTE\n") || block == "NOTE" || strings.HasPrefix(block, "STYLE\n") || strings.HasPrefix(block, "REGION\n") {
			continue
		}
		parts := strings.Split(block, "\n")
		timing := 0
		if !strings.Contains(parts[0], "-->") {
			timing = 1
		}
		if timing >= len(parts)-1 {
			return 0, fmt.Errorf("WebVTT cue needs timing and caption text")
		}
		match := vttTiming.FindStringSubmatch(parts[timing])
		if match == nil {
			return 0, fmt.Errorf("invalid WebVTT cue timing: %q", parts[timing])
		}
		start, err := parseVTTTime(match[1])
		if err != nil {
			return 0, err
		}
		until, err := parseVTTTime(match[2])
		if err != nil {
			return 0, err
		}
		if until <= start || start < previous {
			return 0, fmt.Errorf("WebVTT cues must have positive durations and nondecreasing starts")
		}
		previous = start
		end = math.Max(end, until)
		count++
		if count > 10000 {
			return 0, fmt.Errorf("WebVTT supports at most 10000 cues")
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("caption file contains no cues")
	}
	return end, nil
}

func parseVTTTime(value string) (float64, error) {
	parts := strings.Split(value, ":")
	hours := 0
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("invalid WebVTT timestamp %q", value)
	}
	if len(parts) == 3 {
		var err error
		hours, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid WebVTT timestamp %q", value)
		}
		parts = parts[1:]
	}
	minutes, minuteErr := strconv.Atoi(parts[0])
	seconds, secondErr := strconv.ParseFloat(parts[1], 64)
	if minuteErr != nil || secondErr != nil || minutes < 0 || seconds < 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0, fmt.Errorf("invalid WebVTT timestamp %q", value)
	}
	if minutes > 59 || seconds >= 60 || hours > 999 {
		return 0, fmt.Errorf("invalid WebVTT timestamp %q", value)
	}
	return float64(hours*3600+minutes*60) + seconds, nil
}
