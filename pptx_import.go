package slides

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// PPTXImportReport records what was recovered and what requires author review.
// Import is a content migration, not a layout or animation round trip.
type PPTXImportReport struct {
	Slides      int                 `json:"slides"`
	Images      int                 `json:"images"`
	Destination string              `json:"destination"`
	Warnings    []PPTXImportWarning `json:"warnings"`
}
type PPTXImportWarning struct {
	Slide   int    `json:"slide,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ImportPPTX recovers ordered slide text, tables, cached chart data, notes, and
// PNG/JPEG/GIF assets from common OOXML. The destination must not exist. It never
// fetches external relationships or copies executable, embedded, or vector data.
func ImportPPTX(source, destination string) (PPTXImportReport, error) {
	report := PPTXImportReport{Warnings: []PPTXImportWarning{}}
	dest, err := filepath.Abs(destination)
	if err != nil {
		return report, err
	}
	report.Destination = dest
	if _, err = os.Lstat(dest); err == nil {
		return report, fmt.Errorf("import destination already exists: %s", dest)
	} else if !os.IsNotExist(err) {
		return report, err
	}
	if info, e := os.Stat(filepath.Dir(dest)); e != nil || !info.IsDir() {
		return report, fmt.Errorf("import destination parent must exist")
	}
	p, err := openOfficePackage(source)
	if err != nil {
		return report, err
	}
	defer p.archive.Close()
	part, presentation, err := p.presentation()
	if err != nil {
		return report, err
	}
	rels, err := p.relationships(part)
	if err != nil {
		return report, err
	}
	ids := presentation.descendants("sldId")
	if len(ids) == 0 || len(ids) > 1000 {
		return report, fmt.Errorf("import requires 1–1000 slides")
	}
	stage, err := os.MkdirTemp(filepath.Dir(dest), ".slides-import-*")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(stage)
	if err = os.Mkdir(filepath.Join(stage, "public"), 0755); err != nil {
		return report, err
	}
	addWarning := func(slide int, code, message string) {
		if len(report.Warnings) == 10000 {
			report.Warnings = append(report.Warnings, PPTXImportWarning{Code: "warning-limit", Message: "Further unsupported-content warnings omitted after 10000 entries."})
			return
		}
		if len(report.Warnings) > 10000 {
			return
		}
		if len(message) > 1024 {
			end := 1024
			for end > 0 && !utf8.RuneStart(message[end]) {
				end--
			}
			message = message[:end] + "… [truncated]"
		}
		report.Warnings = append(report.Warnings, PPTXImportWarning{slide, code, message})
	}
	addWarning(0, "layout-content-only", "Imported content needs layout review; original fonts, positions, masters, backgrounds, transitions and animations are not reproduced.")
	for name := range p.files {
		if strings.HasSuffix(strings.ToLower(name), "vbaproject.bin") {
			addWarning(0, "macros-omitted", "Macro content was omitted.")
			break
		}
	}
	var deck strings.Builder
	title := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
	aspect := "16:9"
	if size := presentation.child("sldSz"); size != nil {
		w, e1 := strconv.ParseInt(size.attr("cx"), 10, 64)
		h, e2 := strconv.ParseInt(size.attr("cy"), 10, 64)
		if e1 == nil && e2 == nil && w > 0 && h > 0 && float64(w)/float64(h) >= .5 && float64(w)/float64(h) <= 3 {
			g := officeGCD(w, h)
			aspect = fmt.Sprintf("%d:%d", w/g, h/g)
		} else {
			addWarning(0, "size-defaulted", "Unsupported slide size defaulted to 16:9.")
		}
	}
	fmt.Fprintf(&deck, "---\ntitle: %s\ntheme: paper\naspect-ratio: %s\n---\n\n", strconv.Quote(title), strconv.Quote(aspect))
	images := map[string]string{}
	var imagePixels int64
	for index, id := range ids {
		slideNumber := index + 1
		r, ok := rels[id.relID()]
		if !ok || r.External || !strings.HasSuffix(r.Type, "/slide") {
			return report, fmt.Errorf("slide %d has invalid slide relationship", slideNumber)
		}
		slide, e := p.node(r.Target)
		if e != nil {
			return report, e
		}
		if slide.Name.Local != "sld" {
			return report, fmt.Errorf("slide relationship does not target a slide")
		}
		srels, e := p.relationships(r.Target)
		if e != nil {
			return report, e
		}
		for _, sr := range srels {
			if sr.External {
				addWarning(slideNumber, "external-omitted", "External relationship omitted: "+officeRelationshipKind(sr.Type))
			}
		}
		if index > 0 {
			deck.WriteString("\n---\n\n")
		}
		// HTML blocks keep imported braces and component-looking text literal:
		// migration must never turn untrusted Office prose into GoSX expressions.
		heading := ""
		for _, shape := range slide.descendants("sp") {
			for _, ph := range shape.descendants("ph") {
				if ph.attr("type") == "title" || ph.attr("type") == "ctrTitle" {
					heading = officeParagraphText(shape)
					break
				}
			}
			if heading != "" {
				break
			}
		}
		if heading == "" {
			heading = fmt.Sprintf("Slide %d", slideNumber)
		}
		fmt.Fprintf(&deck, "<h1>%s</h1>\n\n", officeLiteral(heading))
		spTree := slide.descendants("spTree")
		if len(spTree) == 0 {
			return report, fmt.Errorf("slide %d has no shape tree", slideNumber)
		}
		var emit func(*officeNode) error
		emit = func(node *officeNode) error {
			if deck.Len() > 16<<20 {
				return fmt.Errorf("imported Markdown exceeds 16 MiB")
			}
			switch node.Name.Local {
			case "sp":
				text := officeParagraphText(node)
				isTitle := false
				for _, ph := range node.descendants("ph") {
					if ph.attr("type") == "title" || ph.attr("type") == "ctrTitle" {
						isTitle = true
					}
				}
				if text != "" && !isTitle {
					fmt.Fprintf(&deck, "<p>%s</p>\n\n", officeLiteral(text))
				}
				if text == "" {
					addWarning(slideNumber, "shape-omitted", "A non-text shape was omitted.")
				}
			case "pic":
				blips := node.descendants("blip")
				if len(blips) == 0 {
					addWarning(slideNumber, "image-omitted", "Picture has no supported image relationship.")
					return nil
				}
				imageRel, exists := srels[blips[0].attr("embed")]
				if !exists || imageRel.External || !strings.HasSuffix(imageRel.Type, "/image") {
					addWarning(slideNumber, "image-omitted", "Picture relationship is missing or external.")
					return nil
				}
				asset, exists := images[imageRel.Target]
				if !exists {
					data, e := p.read(imageRel.Target, 16<<20)
					if e != nil {
						return e
					}
					config, format, e := image.DecodeConfig(bytes.NewReader(data))
					if e != nil || (format != "png" && format != "jpeg" && format != "gif") || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 32_000_000 || imagePixels+int64(config.Width)*int64(config.Height) > 128_000_000 {
						addWarning(slideNumber, "image-unsupported", "Unsupported or oversized raster image omitted: "+imageRel.Target)
						return nil
					}
					decoded, _, e := image.Decode(bytes.NewReader(data))
					if e != nil {
						addWarning(slideNumber, "image-invalid", "Malformed raster image omitted: "+imageRel.Target)
						return nil
					}
					if format == "gif" || format == "png" && bytes.Contains(data, []byte("acTL")) {
						var snapshot bytes.Buffer
						if e = png.Encode(&snapshot, decoded); e != nil {
							return e
						}
						if snapshot.Len() > 16<<20 {
							addWarning(slideNumber, "image-unsupported", "Raster snapshot exceeds 16 MiB; image omitted.")
							return nil
						}
						data = snapshot.Bytes()
						format = "png"
						addWarning(slideNumber, "image-static", "GIF or animated PNG imported as a static first-frame PNG.")
					}
					ext := format
					if ext == "jpeg" {
						ext = "jpg"
					}
					asset = fmt.Sprintf("imported-image-%d.%s", len(images)+1, ext)
					if e = os.WriteFile(filepath.Join(stage, "public", asset), data, 0644); e != nil {
						return e
					}
					images[imageRel.Target] = asset
					imagePixels += int64(config.Width) * int64(config.Height)
					report.Images++
				}
				alt := "Imported image"
				props := node.descendants("cNvPr")
				if len(props) > 0 {
					if props[0].attr("descr") != "" {
						alt = props[0].attr("descr")
					} else if props[0].attr("name") != "" {
						alt = props[0].attr("name")
					}
				}
				fmt.Fprintf(&deck, "<p><img src=\"/public/%s\" alt=\"%s\"></p>\n\n", asset, html.EscapeString(alt))
				if len(node.descendants("srcRect")) > 0 {
					addWarning(slideNumber, "image-crop-omitted", "Image crop was omitted; original raster content was retained.")
				}
			case "graphicFrame":
				if tables := node.descendants("tbl"); len(tables) > 0 {
					writeOfficeTable(&deck, tables[0])
					for _, cell := range tables[0].descendants("tc") {
						if cell.attr("gridSpan") != "" || cell.attr("rowSpan") != "" || cell.attr("hMerge") == "1" || cell.attr("vMerge") == "1" {
							addWarning(slideNumber, "table-merged-flattened", "Merged table cells were flattened; merge layout requires review.")
							break
						}
					}
					return nil
				}
				if charts := node.descendants("chart"); len(charts) > 0 {
					cr, ok := srels[charts[0].relID()]
					if !ok || cr.External || !strings.HasSuffix(cr.Type, "/chart") {
						addWarning(slideNumber, "chart-omitted", "Chart relationship is unsupported.")
						return nil
					}
					cn, e := p.node(cr.Target)
					if e != nil {
						return e
					}
					if !writeOfficeChart(&deck, cn) {
						addWarning(slideNumber, "chart-omitted", "Chart has no supported cached category/value series.")
					} else {
						addWarning(slideNumber, "chart-data-only", "Supported cached category/value series imported as tables; chart styling, formulas and unsupported series were omitted.")
					}
					return nil
				}
				addWarning(slideNumber, "graphic-omitted", "Unsupported graphic, SmartArt or embedded object was omitted.")
			case "grpSp":
				addWarning(slideNumber, "group-flattened", "Grouped content was flattened into source order.")
				for _, child := range node.Children {
					if e := emit(child); e != nil {
						return e
					}
				}
			case "cxnSp":
				addWarning(slideNumber, "connector-omitted", "A connector was omitted.")
			case "AlternateContent":
				addWarning(slideNumber, "alternate-content-omitted", "Alternate Office content was omitted.")
			case "nvGrpSpPr", "grpSpPr", "extLst":
			default:
				addWarning(slideNumber, "content-omitted", "Unsupported slide content omitted: "+node.Name.Local)
			}
			return nil
		}
		for _, node := range spTree[0].Children {
			if e = emit(node); e != nil {
				return report, e
			}
		}
		if len(slide.descendants("timing")) > 0 || len(slide.descendants("transition")) > 0 {
			addWarning(slideNumber, "motion-omitted", "Slide animations or transitions were omitted.")
		}
		if len(slide.descendants("videoFile"))+len(slide.descendants("audioFile"))+len(slide.descendants("media")) > 0 {
			addWarning(slideNumber, "media-omitted", "Audio/video content was omitted; any supported raster poster image was retained.")
		}
		for _, sr := range srels {
			if strings.HasSuffix(sr.Type, "/notesSlide") && !sr.External {
				notes, e := p.node(sr.Target)
				if e != nil {
					return report, e
				}
				var texts []string
				for _, shape := range notes.descendants("sp") {
					body := false
					for _, ph := range shape.descendants("ph") {
						if ph.attr("type") == "body" {
							body = true
						}
					}
					if body {
						if text := officeParagraphText(shape); text != "" {
							texts = append(texts, text)
						}
					}
				}
				text := strings.Join(texts, "\n")
				if text != "" {
					if strings.Contains(text, "-->") {
						filename := fmt.Sprintf("imported-notes-%d.txt", slideNumber)
						if e = os.WriteFile(filepath.Join(stage, filename), []byte(text), 0600); e != nil {
							return report, e
						}
						addWarning(slideNumber, "notes-delimiter-escaped", "Comment delimiter in notes escaped; original notes retained privately in "+filename)
						text = strings.ReplaceAll(text, "-->", "-- >")
					}
					deck.WriteString("<!-- Imported speaker notes:\n" + text + " -->\n")
				}
			}
		}
		// Ensure mdpp sees the slide separator after an HTML block.
		if !strings.HasSuffix(deck.String(), " -->\n") {
			deck.WriteString("<!-- Imported slide -->\n")
		}
		report.Slides++
		if deck.Len() > 16<<20 {
			return report, fmt.Errorf("imported Markdown exceeds 16 MiB")
		}
	}
	if err = os.WriteFile(filepath.Join(stage, "deck.md"), []byte(deck.String()), 0644); err != nil {
		return report, err
	}
	if err = os.WriteFile(filepath.Join(stage, "go.mod"), []byte(realLaneGoMod(dest)), 0644); err != nil {
		return report, err
	}
	if err = os.WriteFile(filepath.Join(stage, ".gitignore"), []byte("build/\ndist/\n"), 0644); err != nil {
		return report, err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return report, err
	}
	if err = os.WriteFile(filepath.Join(stage, "import-report.json"), append(data, '\n'), 0644); err != nil {
		return report, err
	}
	// Exclusive reservation prevents replacing an existing empty directory even
	// if another process creates it after our initial check.
	if err = os.Mkdir(dest, 0700); err != nil {
		return report, fmt.Errorf("reserve import destination: %w", err)
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		os.RemoveAll(dest)
		return report, err
	}
	for _, entry := range entries {
		if err = os.Rename(filepath.Join(stage, entry.Name()), filepath.Join(dest, entry.Name())); err != nil {
			os.RemoveAll(dest)
			return report, err
		}
	}
	if err = os.Chmod(dest, 0755); err != nil {
		return report, err
	}
	return report, nil
}

func officeRelationshipKind(t string) string { return t[strings.LastIndex(t, "/")+1:] }
func officeGCD(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}
func officeLiteral(text string) string {
	return strings.ReplaceAll(html.EscapeString(text), "\n", "<br>")
}
func officeParagraphText(n *officeNode) string {
	var paragraphs []string
	for _, paragraph := range n.descendants("p") {
		var text strings.Builder
		var walk func(*officeNode)
		walk = func(x *officeNode) {
			if x.Name.Local == "t" {
				text.WriteString(x.Text)
			} else if x.Name.Local == "br" {
				text.WriteByte('\n')
			} else {
				for _, c := range x.Children {
					walk(c)
				}
			}
		}
		walk(paragraph)
		paragraphs = append(paragraphs, text.String())
	}
	return strings.TrimSpace(strings.Join(paragraphs, "\n"))
}
func writeOfficeTable(out *strings.Builder, t *officeNode) {
	out.WriteString("<table>\n")
	for _, row := range t.Children {
		if row.Name.Local != "tr" {
			continue
		}
		out.WriteString("<tr>")
		for _, cell := range row.Children {
			if cell.Name.Local == "tc" {
				fmt.Fprintf(out, "<td>%s</td>", officeLiteral(officeParagraphText(cell)))
			}
		}
		out.WriteString("</tr>\n")
	}
	out.WriteString("</table>\n\n")
}
func officeChartCache(n *officeNode) []string {
	points := n.descendants("pt")
	sort.SliceStable(points, func(i, j int) bool {
		a, _ := strconv.Atoi(points[i].attr("idx"))
		b, _ := strconv.Atoi(points[j].attr("idx"))
		return a < b
	})
	if len(points) > 10000 {
		return nil
	}
	result := make([]string, 0, len(points))
	for i, pt := range points {
		index, err := strconv.Atoi(pt.attr("idx"))
		if err != nil || index != i {
			return nil
		}
		if v := pt.child("v"); v != nil {
			result = append(result, v.Text)
		} else {
			return nil
		}
	}
	return result
}
func writeOfficeChart(out *strings.Builder, n *officeNode) bool {
	var tables strings.Builder
	for _, series := range n.descendants("ser") {
		cat, val := series.child("cat"), series.child("val")
		if cat == nil || val == nil {
			continue
		}
		categories, values := officeChartCache(cat), officeChartCache(val)
		if len(categories) == 0 || len(categories) != len(values) {
			continue
		}
		valid := true
		for _, v := range values {
			f, e := strconv.ParseFloat(v, 64)
			if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				valid = false
			}
		}
		if !valid {
			continue
		}
		tables.WriteString("<table>\n<tr><th>Category</th><th>Value</th></tr>\n")
		for i, c := range categories {
			fmt.Fprintf(&tables, "<tr><td>%s</td><td>%s</td></tr>\n", officeLiteral(c), officeLiteral(values[i]))
		}
		tables.WriteString("</table>\n\n")
	}
	if tables.Len() == 0 {
		return false
	}
	out.WriteString(tables.String())
	return true
}
