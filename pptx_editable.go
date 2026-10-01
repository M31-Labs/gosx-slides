package slides

import (
	"fmt"
	"html"
	"math"
	"regexp"
	"strings"
)

type pptxObject struct {
	Kind, Text, FontFamily, Color, Fill, Stroke string
	X, Y, Width, Height, FontSize, StrokeWidth  float64
	FillAlpha, StrokeAlpha                      *float64
	Bold, Italic, FlipV                         bool
	Path                                        []pptxPathCommand
}
type pptxPathCommand struct {
	Command string
	Points  []float64
}

var pptRGB = regexp.MustCompile(`^[0-9A-Fa-f]{6}$`)

func pptColor(value string) string {
	if pptRGB.MatchString(value) {
		return strings.ToUpper(value)
	}
	return "000000"
}
func pptEMU(px float64) int64 { return int64(math.Round(px * 9525)) }
func pptxObjectsXML(objects []pptxObject) string {
	var out strings.Builder
	for i, o := range objects {
		if o.Width < 0 || o.Height < 0 || math.IsNaN(o.X) || math.IsNaN(o.Y) || math.IsInf(o.X, 0) || math.IsInf(o.Y, 0) {
			continue
		}
		fmt.Fprintf(&out, `<p:sp><p:nvSpPr><p:cNvPr id="%d" name="Editable %s %d"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm`, i+3, html.EscapeString(o.Kind), i+1)
		if o.FlipV {
			out.WriteString(` flipV="1"`)
		}
		fmt.Fprintf(&out, `><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, pptEMU(o.X), pptEMU(o.Y), max(1, pptEMU(o.Width)), max(1, pptEMU(o.Height)))
		if len(o.Path) > 0 {
			out.WriteString(`<a:custGeom><a:avLst/><a:gdLst/><a:ahLst/><a:cxnLst/><a:rect l="0" t="0" r="r" b="b"/><a:pathLst>`)
			fmt.Fprintf(&out, `<a:path w="%d" h="%d">`, max(1, pptEMU(o.Width)), max(1, pptEMU(o.Height)))
			for _, p := range o.Path {
				tag := map[string]string{"M": "moveTo", "L": "lnTo", "C": "cubicBezTo", "Q": "quadBezTo", "Z": "close"}[p.Command]
				if tag == "" {
					continue
				}
				out.WriteString("<a:" + tag + ">")
				for j := 0; j+1 < len(p.Points); j += 2 {
					fmt.Fprintf(&out, `<a:pt x="%d" y="%d"/>`, pptEMU(p.Points[j]), pptEMU(p.Points[j+1]))
				}
				out.WriteString("</a:" + tag + ">")
			}
			out.WriteString(`</a:path></a:pathLst></a:custGeom>`)
		} else {
			geometry := "rect"
			switch o.Kind {
			case "ellipse":
				geometry = "ellipse"
			case "line":
				geometry = "line"
			case "roundRect":
				geometry = "roundRect"
			}
			fmt.Fprintf(&out, `<a:prstGeom prst="%s"><a:avLst/></a:prstGeom>`, geometry)
		}
		if pptRGB.MatchString(o.Fill) {
			fmt.Fprintf(&out, `<a:solidFill><a:srgbClr val="%s">%s</a:srgbClr></a:solidFill>`, pptColor(o.Fill), pptAlpha(o.FillAlpha))
		} else {
			out.WriteString(`<a:noFill/>`)
		}
		if pptRGB.MatchString(o.Stroke) {
			fmt.Fprintf(&out, `<a:ln w="%d"><a:solidFill><a:srgbClr val="%s">%s</a:srgbClr></a:solidFill></a:ln>`, max(1, pptEMU(o.StrokeWidth)), pptColor(o.Stroke), pptAlpha(o.StrokeAlpha))
		} else {
			out.WriteString(`<a:ln><a:noFill/></a:ln>`)
		}
		out.WriteString(`</p:spPr>`)
		if o.Kind == "text" {
			size := int(math.Round(o.FontSize * 75))
			if size < 100 {
				size = 100
			}
			font := o.FontFamily
			if font == "" {
				font = "Arial"
			}
			fmt.Fprintf(&out, `<p:txBody><a:bodyPr wrap="none" lIns="0" rIns="0" tIns="0" bIns="0" anchor="ctr"><a:noAutofit/></a:bodyPr><a:lstStyle/><a:p><a:pPr marL="0" marR="0" indent="0" algn="l"><a:buNone/></a:pPr><a:r><a:rPr lang="en-US" sz="%d" b="%d" i="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="%s"/></a:rPr><a:t xml:space="preserve">%s</a:t></a:r><a:endParaRPr lang="en-US" sz="%d"/></a:p></p:txBody>`, size, boolInt(o.Bold), boolInt(o.Italic), pptColor(o.Color), html.EscapeString(font), html.EscapeString(o.Text), size)
		}
		out.WriteString(`</p:sp>`)
	}
	return out.String()
}
func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func pptAlpha(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf(`<a:alpha val="%d"/>`, int(math.Round(math.Max(0, math.Min(1, *value))*100000)))
}
