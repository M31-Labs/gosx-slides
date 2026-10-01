package slides

import (
	"archive/zip"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PowerPoint uses captured PNG slides so native shaders, typography and 3D
// survive offline viewing. Speaker notes remain editable text in the package.
type pptxWriter struct {
	file    *os.File
	archive *zip.Writer
	path    string
	count   int
}

func newPPTX(path string) (*pptxWriter, error) {
	f, err := os.CreateTemp(filepath.Dir(path), ".slides-pptx-*")
	if err != nil {
		return nil, err
	}
	return &pptxWriter{file: f, archive: zip.NewWriter(f), path: path}, nil
}
func (p *pptxWriter) abort() { p.archive.Close(); p.file.Close(); os.Remove(p.file.Name()) }
func (p *pptxWriter) part(name string, data []byte) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	if strings.HasSuffix(name, ".png") {
		header.Method = zip.Store
	}
	header.SetMode(0644)
	w, err := p.archive.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
func (p *pptxWriter) xml(name, body string) error {
	return p.part(name, []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+body))
}

const pptNamespaces = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`
const pptGroup = `<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>`
const pptColorMap = `<p:clrMap accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" accent5="accent5" accent6="accent6" bg1="lt1" bg2="lt2" folHlink="folHlink" hlink="hlink" tx1="dk1" tx2="dk2"/>`

func pptRels(entries ...string) string {
	return `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + strings.Join(entries, "") + `</Relationships>`
}
func pptRel(id, kind, target string) string {
	return `<Relationship Id="` + id + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/` + kind + `" Target="` + target + `"/>`
}
func (p *pptxWriter) add(png []byte, title, notes string) error {
	return p.addEditable(png, title, notes, nil)
}
func (p *pptxWriter) addEditable(png []byte, title, notes string, objects []pptxObject) error {
	p.count++
	n := p.count
	if err := p.part(fmt.Sprintf("ppt/media/slide%d.png", n), png); err != nil {
		return err
	}
	slide := `<p:sld ` + pptNamespaces + `><p:cSld name="` + html.EscapeString(title) + `"><p:spTree>` + pptGroup + `<p:pic><p:nvPicPr><p:cNvPr id="2" name="` + html.EscapeString(title) + `" descr="Captured slide"/><p:cNvPicPr><a:picLocks noChangeAspect="1"/></p:cNvPicPr><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId1"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="12192000" cy="6858000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>` + pptxObjectsXML(objects) + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
	if err := p.xml(fmt.Sprintf("ppt/slides/slide%d.xml", n), slide); err != nil {
		return err
	}
	if err := p.xml(fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", n), pptRels(pptRel("rId1", "image", fmt.Sprintf("../media/slide%d.png", n)), pptRel("rId2", "slideLayout", "../slideLayouts/slideLayout1.xml"), pptRel("rId3", "notesSlide", fmt.Sprintf("../notesSlides/notesSlide%d.xml", n)))); err != nil {
		return err
	}
	var text strings.Builder
	for _, line := range strings.Split(notes, "\n") {
		text.WriteString(`<a:p><a:r><a:t>` + html.EscapeString(line) + `</a:t></a:r></a:p>`)
	}
	note := `<p:notes ` + pptNamespaces + `><p:cSld><p:spTree>` + pptGroup + `<p:sp><p:nvSpPr><p:cNvPr id="2" name="Speaker notes"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>` + text.String() + `</p:txBody></p:sp></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:notes>`
	if err := p.xml(fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", n), note); err != nil {
		return err
	}
	return p.xml(fmt.Sprintf("ppt/notesSlides/_rels/notesSlide%d.xml.rels", n), pptRels(pptRel("rId1", "notesMaster", "../notesMasters/notesMaster1.xml"), pptRel("rId2", "slide", fmt.Sprintf("../slides/slide%d.xml", n))))
}
func (p *pptxWriter) finish(title string) error {
	var slides, overrides strings.Builder
	rels := []string{pptRel("rId1", "slideMaster", "slideMasters/slideMaster1.xml"), pptRel("rId2", "notesMaster", "notesMasters/notesMaster1.xml")}
	for i := 1; i <= p.count; i++ {
		fmt.Fprintf(&slides, `<p:sldId id="%d" r:id="rId%d"/>`, 255+i, i+2)
		rels = append(rels, pptRel(fmt.Sprintf("rId%d", i+2), "slide", fmt.Sprintf("slides/slide%d.xml", i)))
		for _, kind := range []string{"slides/slide", "notesSlides/notesSlide"} {
			ct := "slide"
			if strings.HasPrefix(kind, "notes") {
				ct = "notesSlide"
			}
			fmt.Fprintf(&overrides, `<Override PartName="/ppt/%s%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.%s+xml"/>`, kind, i, ct)
		}
	}
	parts := map[string]string{
		"_rels/.rels":                                  pptRels(pptRel("rId1", "officeDocument", "ppt/presentation.xml"), `<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>`),
		"docProps/core.xml":                            `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>` + html.EscapeString(title) + `</dc:title><dc:creator>gosx-slides</dc:creator></cp:coreProperties>`,
		"ppt/presentation.xml":                         `<p:presentation ` + pptNamespaces + `><p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst><p:notesMasterIdLst><p:notesMasterId r:id="rId2"/></p:notesMasterIdLst><p:sldIdLst>` + slides.String() + `</p:sldIdLst><p:sldSz cx="12192000" cy="6858000" type="screen16x9"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels":              pptRels(rels...),
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster ` + pptNamespaces + `><p:cSld><p:spTree>` + pptGroup + `</p:spTree></p:cSld>` + pptColorMap + `<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst><p:txStyles><p:titleStyle/><p:bodyStyle/><p:otherStyle/></p:txStyles></p:sldMaster>`,
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": pptRels(pptRel("rId1", "slideLayout", "../slideLayouts/slideLayout1.xml"), pptRel("rId2", "theme", "../theme/theme1.xml")),
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout ` + pptNamespaces + ` type="blank" preserve="1"><p:cSld name="Blank"><p:spTree>` + pptGroup + `</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": pptRels(pptRel("rId1", "slideMaster", "../slideMasters/slideMaster1.xml")),
		"ppt/notesMasters/notesMaster1.xml":            `<p:notesMaster ` + pptNamespaces + `><p:cSld><p:spTree>` + pptGroup + `</p:spTree></p:cSld>` + pptColorMap + `<p:notesStyle/></p:notesMaster>`,
		"ppt/notesMasters/_rels/notesMaster1.xml.rels": pptRels(pptRel("rId1", "theme", "../theme/theme1.xml")),
		"ppt/theme/theme1.xml":                         pptTheme,
	}
	ct := `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="png" ContentType="image/png"/><Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`
	for _, part := range []struct{ path, kind string }{{"presentation", "presentation.main"}, {"slideMasters/slideMaster1", "slideMaster"}, {"slideLayouts/slideLayout1", "slideLayout"}, {"notesMasters/notesMaster1", "notesMaster"}, {"theme/theme1", "theme"}} {
		mime := "application/vnd.openxmlformats-officedocument.presentationml." + part.kind + "+xml"
		if part.kind == "theme" {
			mime = "application/vnd.openxmlformats-officedocument.theme+xml"
		}
		ct += `<Override PartName="/ppt/` + part.path + `.xml" ContentType="` + mime + `"/>`
	}
	parts["[Content_Types].xml"] = ct + overrides.String() + `</Types>`
	// Stable part order makes the package reproducible for the same captures.
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := p.xml(name, parts[name]); err != nil {
			return err
		}
	}
	if err := p.archive.Close(); err != nil {
		return err
	}
	if err := p.file.Sync(); err != nil {
		return err
	}
	if err := p.file.Close(); err != nil {
		return err
	}
	return os.Rename(p.file.Name(), p.path)
}

const pptTheme = `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="GoSX"><a:themeElements><a:clrScheme name="GoSX"><a:dk1><a:srgbClr val="000000"/></a:dk1><a:lt1><a:srgbClr val="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="151923"/></a:dk2><a:lt2><a:srgbClr val="F1F3F8"/></a:lt2><a:accent1><a:srgbClr val="6B9CFF"/></a:accent1><a:accent2><a:srgbClr val="FF8A65"/></a:accent2><a:accent3><a:srgbClr val="64D8B4"/></a:accent3><a:accent4><a:srgbClr val="B69CFF"/></a:accent4><a:accent5><a:srgbClr val="FFD166"/></a:accent5><a:accent6><a:srgbClr val="EC83B4"/></a:accent6><a:hlink><a:srgbClr val="0000FF"/></a:hlink><a:folHlink><a:srgbClr val="800080"/></a:folHlink></a:clrScheme><a:fontScheme name="GoSX"><a:majorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="Arial"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme><a:fmtScheme name="GoSX"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst><a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="12700"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln w="19050"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst><a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst><a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>`
