package slides

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
)

func validPPTXChart(c *pptxChart) bool {
	if c == nil || (c.Type != "bar" && c.Type != "pie") || len(c.Values) == 0 || len(c.Values) > 128 || len(c.Values) != len(c.Categories) {
		return false
	}
	sum := 0.0
	for i, v := range c.Values {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1e15 || len(c.Categories[i]) > 1024 || c.Type == "pie" && v < 0 {
			return false
		}
		sum += v
	}
	return c.Type != "pie" || sum > 0
}
func pptxChartFrameXML(id int, relID string, o pptxObject) string {
	return fmt.Sprintf(`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="%d" name="Editable chart data"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr><p:xfrm><a:off x="%d" y="%d"/><a:ext cx="%d" cy="%d"/></p:xfrm><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="%s"/></a:graphicData></a:graphic></p:graphicFrame>`, id, pptEMU(o.X), pptEMU(o.Y), pptEMU(o.Width), pptEMU(o.Height), relID)
}
func (p *pptxWriter) addChart(id int, c *pptxChart) error {
	var categories, values, colors strings.Builder
	for i, v := range c.Values {
		fmt.Fprintf(&categories, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, html.EscapeString(c.Categories[i]))
		fmt.Fprintf(&values, `<c:pt idx="%d"><c:v>%s</c:v></c:pt>`, i, strconv.FormatFloat(v, 'g', -1, 64))
		if i < len(c.Colors) && pptRGB.MatchString(c.Colors[i]) {
			fmt.Fprintf(&colors, `<c:dPt><c:idx val="%d"/><c:spPr><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:ln><a:noFill/></a:ln></c:spPr></c:dPt>`, i, pptColor(c.Colors[i]))
		}
	}
	n := len(c.Values)
	series := fmt.Sprintf(`<c:ser><c:idx val="0"/><c:order val="0"/><c:tx><c:v>Value</c:v></c:tx>%s<c:cat><c:strRef><c:f>Sheet1!$A$2:$A$%d</c:f><c:strCache><c:ptCount val="%d"/>%s</c:strCache></c:strRef></c:cat><c:val><c:numRef><c:f>Sheet1!$B$2:$B$%d</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="%d"/>%s</c:numCache></c:numRef></c:val></c:ser>`, colors.String(), n+1, n, categories.String(), n+1, n, values.String())
	plot := `<c:pieChart><c:varyColors val="1"/>` + series + `<c:firstSliceAng val="270"/></c:pieChart>`
	if c.Type == "bar" {
		plot = `<c:barChart><c:barDir val="bar"/><c:grouping val="clustered"/><c:varyColors val="1"/>` + series + `<c:gapWidth val="100"/><c:axId val="1"/><c:axId val="2"/></c:barChart><c:catAx><c:axId val="1"/><c:scaling><c:orientation val="maxMin"/></c:scaling><c:delete val="0"/><c:axPos val="l"/><c:tickLblPos val="nextTo"/><c:crossAx val="2"/><c:crosses val="autoZero"/><c:auto val="1"/><c:lblAlgn val="ctr"/><c:lblOffset val="100"/></c:catAx><c:valAx><c:axId val="2"/><c:scaling><c:orientation val="minMax"/></c:scaling><c:delete val="0"/><c:axPos val="b"/><c:numFmt formatCode="General" sourceLinked="1"/><c:tickLblPos val="nextTo"/><c:crossAx val="1"/><c:crosses val="max"/><c:crossBetween val="between"/></c:valAx>`
	}
	legend := ""
	if c.Type == "pie" {
		legend = `<c:legend><c:legendPos val="r"/><c:overlay val="0"/></c:legend>`
	}
	font := c.FontFamily
	if font == "" {
		font = "+mn-lt"
	}
	size := 1200
	if c.FontSize > 0 && c.FontSize <= 512 {
		size = int(math.Round(c.FontSize * 75))
	}
	textStyle := fmt.Sprintf(`<c:txPr><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr sz="%d"><a:solidFill><a:srgbClr val="%s"/></a:solidFill><a:latin typeface="%s"/></a:defRPr></a:pPr><a:endParaRPr/></a:p></c:txPr>`, size, pptColor(c.TextColor), html.EscapeString(font))
	chart := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><c:date1904 val="0"/><c:lang val="en-US"/><c:chart><c:autoTitleDeleted val="1"/><c:plotArea><c:layout/>` + plot + `</c:plotArea>` + legend + `<c:plotVisOnly val="1"/><c:dispBlanksAs val="gap"/></c:chart><c:spPr><a:noFill/><a:ln><a:noFill/></a:ln></c:spPr>` + textStyle + `<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData></c:chartSpace>`
	if err := p.xml(fmt.Sprintf("ppt/charts/chart%d.xml", id), chart); err != nil {
		return err
	}
	if err := p.xml(fmt.Sprintf("ppt/charts/_rels/chart%d.xml.rels", id), pptRels(pptRel("rId1", "package", fmt.Sprintf("../embeddings/chart%d.xlsx", id)))); err != nil {
		return err
	}
	workbook, err := pptxChartWorkbook(c)
	if err != nil {
		return err
	}
	return p.part(fmt.Sprintf("ppt/embeddings/chart%d.xlsx", id), workbook)
}

// A real embedded workbook makes PowerPoint's Edit Data action available;
// cached chart values alone only preserve visual rendering.
func pptxChartWorkbook(c *pptxChart) ([]byte, error) {
	var data bytes.Buffer
	z := zip.NewWriter(&data)
	parts := []struct{ name, xml string }{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", pptRels(pptRel("rId1", "officeDocument", "xl/workbook.xml"))},
		{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", pptRels(pptRel("rId1", "worksheet", "worksheets/sheet1.xml"))},
	}
	var sheet strings.Builder
	fmt.Fprintf(&sheet, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="A1:B%d"/><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>Category</t></is></c><c r="B1" t="inlineStr"><is><t>Value</t></is></c></row>`, len(c.Values)+1)
	for i, v := range c.Values {
		fmt.Fprintf(&sheet, `<row r="%d"><c r="A%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c><c r="B%d"><v>%s</v></c></row>`, i+2, i+2, html.EscapeString(c.Categories[i]), i+2, strconv.FormatFloat(v, 'g', -1, 64))
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	parts = append(parts, struct{ name, xml string }{"xl/worksheets/sheet1.xml", sheet.String()})
	for _, part := range parts {
		w, err := z.Create(part.name)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + part.xml)); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
