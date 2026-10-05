package slides

import (
	"fmt"
	"html"
	"math"
	"strings"
)

func pptxTableXML(id int, o pptxObject) string {
	t := o.Table
	if t == nil || len(t.Rows) == 0 || len(t.Rows) > 64 || len(t.ColumnWidths) == 0 || len(t.ColumnWidths) > 16 || len(t.RowHeights) != len(t.Rows) {
		return ""
	}
	for _, row := range t.Rows {
		if len(row) != len(t.ColumnWidths) {
			return ""
		}
	}
	for _, sizes := range [][]float64{t.ColumnWidths, t.RowHeights} {
		for _, v := range sizes {
			if !(v > 0 && v <= 4096) {
				return ""
			}
		}
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Editable table"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table"><a:tbl><a:tblPr/><a:tblGrid>`, id, pptEMU(o.X), pptEMU(o.Y), pptEMU(o.Width), pptEMU(o.Height))
	for _, w := range t.ColumnWidths {
		fmt.Fprintf(&out, `<a:gridCol w="%d"/>`, pptEMU(w))
	}
	out.WriteString(`</a:tblGrid>`)
	for r, row := range t.Rows {
		fmt.Fprintf(&out, `<a:tr h="%d">`, pptEMU(t.RowHeights[r]))
		for _, c := range row {
			for _, v := range append([]float64{c.FontSize, c.BorderWidth, c.Padding}, c.Margins...) {
				if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 4096 {
					return ""
				}
			}
			for _, b := range c.Borders {
				if !(b.Width >= 0 && b.Width <= 100) {
					return ""
				}
			}
			if len(c.Text) > 20000 {
				return ""
			}
			size := int(math.Round(math.Max(1, math.Min(512, c.FontSize)) * 75))
			font := c.FontFamily
			if font == "" {
				font = "Arial"
			}
			align := map[string]string{"left": "l", "center": "ctr", "right": "r", "start": "l", "end": "r"}[c.Align]
			if align == "" {
				align = "l"
			}
			out.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/>`)
			for _, line := range strings.Split(c.Text, "\n") {
				fmt.Fprintf(&out, `<a:p><a:pPr algn="%s"><a:buNone/></a:pPr><a:r><a:rPr lang="en-US" sz="%d" b="%d" i="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="%s"/></a:rPr><a:t xml:space="preserve">%s</a:t></a:r><a:endParaRPr sz="%d"/></a:p>`, align, size, boolInt(c.Bold), boolInt(c.Italic), pptColor(c.Color), html.EscapeString(font), html.EscapeString(line), size)
			}
			margins := []float64{c.Padding, c.Padding, c.Padding, c.Padding}
			if len(c.Margins) == 4 {
				margins = c.Margins
			}
			fmt.Fprintf(&out, `</a:txBody><a:tcPr marL="%d" marR="%d" marT="%d" marB="%d" anchor="ctr">`, pptEMU(margins[0]), pptEMU(margins[1]), pptEMU(margins[2]), pptEMU(margins[3]))
			for i, side := range []string{"L", "R", "T", "B"} {
				border := pptxBorder{Color: c.Border, Width: c.BorderWidth}
				if len(c.Borders) == 4 {
					border = c.Borders[i]
				}
				fmt.Fprintf(&out, `<a:ln%s w="%d">`, side, max(1, pptEMU(border.Width)))
				if border.Width > 0 && pptRGB.MatchString(border.Color) {
					fmt.Fprintf(&out, `<a:solidFill><a:srgbClr val="%s">%s</a:srgbClr></a:solidFill>`, pptColor(border.Color), pptAlpha(border.Alpha))
				} else {
					out.WriteString(`<a:noFill/>`)
				}
				out.WriteString(`<a:prstDash val="solid"/></a:ln` + side + `>`)
			}
			if pptRGB.MatchString(c.Fill) {
				fmt.Fprintf(&out, `<a:solidFill><a:srgbClr val="%s">%s</a:srgbClr></a:solidFill>`, pptColor(c.Fill), pptAlpha(c.FillAlpha))
			} else {
				out.WriteString(`<a:noFill/>`)
			}
			out.WriteString(`</a:tcPr></a:tc>`)
		}
		out.WriteString(`</a:tr>`)
	}
	out.WriteString(`</a:tbl></a:graphicData></a:graphic></p:graphicFrame>`)
	return out.String()
}
