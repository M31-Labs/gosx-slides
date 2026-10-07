package slides

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

var netDialer = net.Dialer{Timeout: 10 * time.Second}

// Chrome's local DevTools protocol keeps graphic capture pure Go. Neither Node
// nor Playwright is required to export; they are only browser-test dependencies.
type captureBrowser struct {
	conn    *websocket.Conn
	id      int
	cmd     *exec.Cmd
	tmp     string
	onEvent func(string, json.RawMessage)
}

func findChrome() (string, error) {
	if chrome := os.Getenv("SLIDES_CHROME"); chrome != "" {
		return chrome, nil
	}
	for _, candidate := range pdfChromeCandidates {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("capture export needs Chrome or Chromium on PATH (or SLIDES_CHROME=/path/to/chrome)")
}
func startCaptureBrowser(ctx context.Context) (*captureBrowser, error) {
	chrome, err := findChrome()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "slides-capture-*")
	if err != nil {
		return nil, err
	}
	b := &captureBrowser{tmp: tmp}
	b.cmd = exec.CommandContext(ctx, chrome, "--headless=new", "--mute-audio", "--autoplay-policy=user-gesture-required", "--no-first-run", "--no-default-browser-check", "--enable-unsafe-swiftshader", "--remote-debugging-address=127.0.0.1", "--remote-allow-origins=http://127.0.0.1", "--remote-debugging-port=0", "--user-data-dir="+tmp, "about:blank")
	var logs bytes.Buffer
	b.cmd.Stderr = &logs
	if err = b.cmd.Start(); err != nil {
		os.RemoveAll(tmp)
		return nil, err
	}
	failed := func(err error) (*captureBrowser, error) { b.close(); return nil, err }
	var port string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(filepath.Join(tmp, "DevToolsActivePort"))
		if err == nil {
			port = strings.SplitN(string(data), "\n", 2)[0]
			break
		}
		select {
		case <-ctx.Done():
			return failed(ctx.Err())
		case <-time.After(50 * time.Millisecond):
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		return failed(fmt.Errorf("Chrome did not start its capture endpoint; check SLIDES_CHROME"))
	}
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, "http://127.0.0.1:"+port+"/json/new?about:blank", nil)
	response, err := client.Do(req)
	if err != nil {
		return failed(err)
	}
	var target struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	err = json.NewDecoder(response.Body).Decode(&target)
	response.Body.Close()
	if err != nil || target.WebSocketDebuggerURL == "" {
		return failed(fmt.Errorf("Chrome did not return a capture target"))
	}
	config, err := websocket.NewConfig(target.WebSocketDebuggerURL, "http://127.0.0.1")
	if err != nil {
		return failed(err)
	}
	config.Dialer = &netDialer
	b.conn, err = websocket.DialConfig(config)
	if err != nil {
		return failed(err)
	}
	b.conn.MaxPayloadBytes = 128 << 20
	return b, nil
}
func (b *captureBrowser) close() {
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	if b.cmd != nil && b.cmd.Process != nil {
		b.cmd.Process.Kill()
		b.cmd.Wait()
		b.cmd = nil
	}
	if b.tmp != "" {
		os.RemoveAll(b.tmp)
	}
}
func (b *captureBrowser) call(method string, params any, out any) error {
	b.id++
	b.conn.SetDeadline(time.Now().Add(45 * time.Second))
	if err := websocket.JSON.Send(b.conn, map[string]any{"id": b.id, "method": method, "params": params}); err != nil {
		return err
	}
	for {
		var response struct {
			ID     int             `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := websocket.JSON.Receive(b.conn, &response); err != nil {
			return err
		}
		if response.ID != b.id {
			if b.onEvent != nil && response.Method != "" {
				b.onEvent(response.Method, response.Params)
			}
			continue
		}
		if response.Error != nil {
			return fmt.Errorf("Chrome %s: %s", method, response.Error.Message)
		}
		if out != nil {
			return json.Unmarshal(response.Result, out)
		}
		return nil
	}
}
func (b *captureBrowser) eval(expression string, out any) error {
	var result struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		Exception json.RawMessage `json:"exceptionDetails"`
	}
	if err := b.call("Runtime.evaluate", map[string]any{"expression": expression, "awaitPromise": true, "returnByValue": true}, &result); err != nil {
		return err
	}
	if len(result.Exception) > 0 {
		return fmt.Errorf("capture page failed: %.1000s", result.Exception)
	}
	if out != nil {
		return json.Unmarshal(result.Result.Value, out)
	}
	return nil
}
func (b *captureBrowser) wait(expression string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		var ready bool
		if err := b.eval(expression, &ready); err == nil && ready {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("capture page did not become ready")
}

func (b *captureBrowser) png() ([]byte, error) {
	var result struct {
		Data string `json:"data"`
	}
	if err := b.call("Page.captureScreenshot", map[string]any{"format": "png", "captureBeyondViewport": false}, &result); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(result.Data)
}

const captureReady = `async function waitFor(test) {
 const deadline = Date.now() + 25000;
 while (!test()) { if (Date.now() > deadline) throw new Error('Slide runtime or graphic did not become ready'); await new Promise(r => setTimeout(r, 50)); }
}`

// Capture exports use the same served runtime, including shaders and Scene3D.
// Every selected state is visited and checked before pixels are read.
func exportCaptured(deck *IslandDeck, opts ExportOptions) error {
	narration, err := prepareVideoNarration(context.Background(), deck, opts)
	if err != nil {
		return err
	}
	defer narration.close()
	width, height, err := exportSize(deck, opts)
	if err != nil {
		return err
	}
	app, err := deck.NewServer(ServeOptions{StageRuntime: true, Static: true, IncludeNotes: opts.Notes})
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("/", app.Build())
	var snapshot atomic.Value
	snapshot.Store("")
	mux.HandleFunc("/_slides-capture.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, snapshot.Load().(string))
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	browser, err := startCaptureBrowser(ctx)
	if err != nil {
		return err
	}
	defer browser.close()
	if err = browser.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": height, "deviceScaleFactor": 1, "mobile": false}, nil); err != nil {
		return err
	}
	if err = browser.call("Page.navigate", map[string]any{"url": server.URL}, nil); err != nil {
		return err
	}
	if err = browser.wait(`document.readyState === "complete" && !!window.SlidesNav && (!document.getElementById("gosx-manifest") || !!(window.__gosx && window.__gosx.ready))`); err != nil {
		return err
	}
	if err = browser.eval(`(async()=>{`+captureReady+`;await waitFor(()=>window.SlidesNav && document.readyState==='complete' && (!document.getElementById('gosx-manifest') || window.__gosx && window.__gosx.ready));if(document.fonts)await document.fonts.ready;
 const css=document.createElement('style');css.textContent='.deck-controls,.deck-counter,.deck-progress,.deck-overflow-badge,.code-copy{display:none!important}';document.head.appendChild(css);return true;})()`, nil); err != nil {
		return err
	}
	out := opts.OutDir
	if out == "" {
		out = "dist"
	}
	framesDir := out
	if opts.Format == "pdf" || opts.Format == "video" || opts.Format == "pptx" {
		framesDir = filepath.Dir(out)
		if filepath.Ext(out) == "" {
			framesDir = out
		}
	}
	if err = os.MkdirAll(framesDir, 0755); err != nil {
		return err
	}
	var pptx *pptxWriter
	if opts.Format == "pptx" {
		path := out
		if filepath.Ext(out) == "" {
			path = filepath.Join(out, "deck.pptx")
		}
		pptx, err = newPPTXConfigured(path, width, height, opts.PPTXTemplate)
		if err != nil {
			return err
		}
		defer pptx.abort()
	}
	var pages strings.Builder
	pages.WriteString(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + html.EscapeString(deck.title()) + `</title><style>` + navStyle() + presentationControlsStyle() + authoringStyle + officePageCSS(width, height) + `main.deck>.slide{position:relative;padding:0!important;background:#000}main.deck .capture-frame{display:block;width:100%;height:100vh;max-height:none;object-fit:contain;margin:0}.capture-description{position:absolute;top:0;left:0;margin:0;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}@media print{main.deck>.slide:last-of-type{break-after:auto;page-break-after:auto}}</style></head><body><main class="deck" data-transition="none" data-live-sync="0">`)
	var video *exec.Cmd
	var pipe io.WriteCloser
	var videoLog bytes.Buffer
	videoFrames := 0
	videoOutput, videoFFmpeg := "", ""
	var videoCaptions []videoCaption
	if opts.Format == "video" {
		ffmpeg, err := exec.LookPath("ffmpeg")
		if err != nil {
			return fmt.Errorf("video export needs ffmpeg on PATH")
		}
		path := out
		if filepath.Ext(out) == "" {
			path = filepath.Join(out, "deck.webm")
		}
		videoOutput, videoFFmpeg = path, ffmpeg
		if err = narration.stageVideo(path); err != nil {
			return err
		}
		path = narration.video
		video = exec.CommandContext(ctx, ffmpeg, "-y", "-hide_banner", "-loglevel", "error", "-f", "image2pipe", "-framerate", strconv.Itoa(opts.FPS), "-vcodec", "png", "-i", "pipe:0", "-an", "-c:v", "libvpx-vp9", "-pix_fmt", "yuv420p", "-deadline", "realtime", path)
		video.Stderr = &videoLog
		pipe, err = video.StdinPipe()
		if err != nil {
			return err
		}
		if err = video.Start(); err != nil {
			return err
		}
		defer func() {
			pipe.Close()
			if video.Process != nil {
				video.Process.Kill()
				video.Wait()
			}
		}()
	}
	pageCount := 0
	for i, slide := range deck.Slides {
		budget := 0
		if err = browser.eval(fmt.Sprintf(`SlidesNav.show(%d,0,false);SlidesNav.stepCount()`, i), &budget); err != nil {
			return err
		}
		last := 0
		if opts.Steps {
			last = budget
		}
		if last > 10000 || pageCount+last+1 > 10000 {
			return fmt.Errorf("capture export supports at most 10000 slide states")
		}
		for step := 0; step <= last; step++ {
			expression := fmt.Sprintf(`(async()=>{`+captureReady+`;SlidesNav.show(%d,%d,false);await waitFor(()=>{const slide=document.querySelector('.deck-active');return slide.dataset.slideHydration!=='pending' && Array.from(slide.querySelectorAll('.slide-graphic[data-slide-steps]')).every(m=>Number(m.dataset.appliedStep)===Math.min(%d,JSON.parse(m.dataset.slideSteps).frames.length-1)) && Array.from(slide.querySelectorAll('.slide-graphic')).every(m=>m.__gosxScene3DHandle && m.__gosxScene3DHandle.__gosxScene3DCommandReady) && Array.from(slide.querySelectorAll('img')).every(img=>img.complete);});await document.fonts.ready;await new Promise(r=>setTimeout(r,200));return true;})()`, i, step, step)
			if err = browser.eval(expression, nil); err != nil {
				return fmt.Errorf("capture slide %d step %d: %w", i+1, step, err)
			}
			if opts.Format == "video" {
				frameCount := int(math.Ceil(opts.Seconds * float64(opts.FPS)))
				videoCaptions = append(videoCaptions, videoCaption{StartMS: videoFrames * 1000 / opts.FPS, EndMS: (videoFrames + frameCount) * 1000 / opts.FPS, Text: deckRecordingCaption(deck, slide, step)})
				videoFrames += frameCount
				if videoFrames > 18000 {
					return fmt.Errorf("video export supports at most 18000 frames; reduce --seconds, --fps, or steps")
				}
				start := time.Now()
				for frame := 0; frame < frameCount; frame++ {
					if err = browser.eval(fmt.Sprintf(`(async()=>{if(window.SlidesMotion){SlidesMotion.seek(%d);await SlidesMotion.settled();}return true;})()`, frame*1000/opts.FPS), nil); err != nil {
						return err
					}
					pixels, err := browser.png()
					if err != nil {
						return err
					}
					if _, err = pipe.Write(pixels); err != nil {
						pipe.Close()
						video.Process.Kill()
						video.Wait()
						return fmt.Errorf("ffmpeg: %w: %.1000s", err, videoLog.String())
					}
					target := start.Add(time.Duration(float64(frame+1) / float64(opts.FPS) * float64(time.Second)))
					if wait := time.Until(target); wait > 0 {
						time.Sleep(wait)
					}
				}
				continue
			}
			if err = browser.eval(`(async()=>{if(window.SlidesMotion){SlidesMotion.seek(SlidesMotion.duration());await SlidesMotion.settled();}return true;})()`, nil); err != nil {
				return err
			}
			var editableObjects []pptxObject
			if pptx != nil && opts.Editable {
				if err = browser.eval(pptxEditableScript, &editableObjects); err != nil {
					return err
				}
			}
			pixels, err := browser.png()
			if pptx != nil && opts.Editable {
				restoreErr := browser.eval(`window.__slidesPPTXRestore?.();true`, nil)
				if err == nil {
					err = restoreErr
				}
			}
			if err != nil {
				return err
			}
			if opts.Format == "frames" {
				name := fmt.Sprintf("slide-%03d-step-%03d.png", i+1, step)
				if err = os.WriteFile(filepath.Join(framesDir, name), pixels, 0644); err != nil {
					return err
				}
				pageCount++
				continue
			}
			label := slideTitle(slide)
			if opts.Steps {
				label += fmt.Sprintf(" — step %d", step)
			}
			if pptx != nil {
				notes := ""
				if opts.Notes {
					notes = extractSlideNotes(slide)
				}
				if err = pptx.addEditable(pixels, label, notes, editableObjects); err != nil {
					return err
				}
				pageCount++
				continue
			}
			fmt.Fprintf(&pages, `<section class="slide" data-slide="%d"><img class="capture-frame" alt="%s" src="data:image/png;base64,%s"><p class="capture-description">%s</p></section>`, pageCount, html.EscapeString(label), base64.StdEncoding.EncodeToString(pixels), html.EscapeString(slidePlainText(slide)))
			pageCount++
		}
	}
	if video != nil {
		pipe.Close()
		if err = video.Wait(); err != nil {
			return fmt.Errorf("ffmpeg video encode: %w: %.1000s", err, videoLog.String())
		}
		return narration.finish(ctx, videoFFmpeg, videoOutput, float64(videoFrames)/float64(opts.FPS), videoCaptions)
	}
	if pptx != nil {
		return pptx.finish(deck.title())
	}
	pages.WriteString(`<script>` + navScript() + `</script></main></body></html>`)
	if opts.Format == "frames" {
		return nil
	}
	if opts.Format == "single" {
		return os.WriteFile(filepath.Join(out, "deck.html"), []byte(pages.String()), 0644)
	}
	snapshot.Store(pages.String())
	if err = browser.call("Page.navigate", map[string]any{"url": server.URL + "/_slides-capture.html"}, nil); err != nil {
		return err
	}
	if err = browser.wait(`location.pathname === "/_slides-capture.html" && document.readyState === "complete"`); err != nil {
		return err
	}
	if err = browser.eval(`(async()=>{`+captureReady+`;await waitFor(()=>document.readyState==='complete' && Array.from(document.images).every(i=>i.complete));return true;})()`, nil); err != nil {
		return err
	}
	var pdf struct {
		Data string `json:"data"`
	}
	if err = browser.call("Page.printToPDF", map[string]any{"printBackground": true, "preferCSSPageSize": true, "displayHeaderFooter": false}, &pdf); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(pdf.Data)
	if err != nil {
		return err
	}
	path := out
	if !strings.EqualFold(filepath.Ext(out), ".pdf") {
		path = filepath.Join(out, "deck.pdf")
	}
	return os.WriteFile(path, data, 0644)
}
