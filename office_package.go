package slides

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

// Office input is read in place, never extracted. Bounds apply even to unused
// entries so a template or imported archive cannot smuggle an unbounded ZIP.
const officeMaxXML = 4 << 20

type officePackage struct {
	archive   *zip.ReadCloser
	files     map[string]*zip.File
	readBytes int64
}

func openOfficePackage(filename string) (*officePackage, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 100<<20 {
		return nil, fmt.Errorf("Office input must be a regular file of at most 100 MiB")
	}
	z, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	p := &officePackage{archive: z, files: map[string]*zip.File{}}
	var total uint64
	for _, f := range z.File {
		name := strings.TrimSuffix(f.Name, "/")
		if len(z.File) > 10000 || name == "" || strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") || path.Clean(name) != name || strings.HasPrefix(name, "../") || f.Mode()&os.ModeSymlink != 0 {
			z.Close()
			return nil, fmt.Errorf("unsafe Office archive entry %q", f.Name)
		}
		if _, exists := p.files[name]; exists {
			z.Close()
			return nil, fmt.Errorf("duplicate Office archive entry %q", name)
		}
		if f.UncompressedSize64 > 16<<20 || total > 128<<20-f.UncompressedSize64 {
			z.Close()
			return nil, fmt.Errorf("Office archive exceeds decompressed size limits")
		}
		total += f.UncompressedSize64
		p.files[name] = f
	}
	return p, nil
}

func (p *officePackage) read(name string, limit int64) ([]byte, error) {
	f := p.files[name]
	if f == nil {
		return nil, fmt.Errorf("missing Office part %q", name)
	}
	if f.UncompressedSize64 > uint64(limit) {
		return nil, fmt.Errorf("Office part %q exceeds size limit", name)
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("Office part %q exceeds size limit", name)
	}
	if p.readBytes > 128<<20-int64(len(data)) {
		return nil, fmt.Errorf("Office processing exceeds 128 MiB read budget")
	}
	p.readBytes += int64(len(data))
	return data, nil
}

type officeNode struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Text     string
	Children []*officeNode
	text     strings.Builder
}

func parseOfficeXML(data []byte) (*officeNode, error) {
	if len(data) > officeMaxXML {
		return nil, fmt.Errorf("Office XML exceeds 4 MiB")
	}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	root := &officeNode{}
	stack := []*officeNode{root}
	nodes, tokens := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		tokens++
		if tokens > 300000 {
			return nil, fmt.Errorf("Office XML exceeds token limit")
		}
		switch token := token.(type) {
		case xml.StartElement:
			nodes++
			if nodes > 100000 || len(stack) > 64 {
				return nil, fmt.Errorf("Office XML exceeds node or nesting limit")
			}
			n := &officeNode{Name: token.Name, Attrs: token.Attr}
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) < 2 {
				return nil, fmt.Errorf("invalid Office XML nesting")
			}
			stack[len(stack)-1].Text = stack[len(stack)-1].text.String()
			stack = stack[:len(stack)-1]
		case xml.CharData:
			stack[len(stack)-1].text.Write(token)
		case xml.Directive:
			return nil, fmt.Errorf("Office XML directives are not supported")
		case xml.ProcInst:
			if token.Target != "xml" {
				return nil, fmt.Errorf("Office XML processing instructions are not supported")
			}
		}
	}
	if len(root.Children) != 1 || strings.TrimSpace(root.text.String()) != "" {
		return nil, fmt.Errorf("Office XML must have one root")
	}
	return root.Children[0], nil
}
func (p *officePackage) node(name string) (*officeNode, error) {
	data, err := p.read(name, officeMaxXML)
	if err != nil {
		return nil, err
	}
	return parseOfficeXML(data)
}
func (n *officeNode) attr(key string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == key {
			return a.Value
		}
	}
	return ""
}
func (n *officeNode) relID() string {
	for _, a := range n.Attrs {
		if a.Name.Local == "id" && strings.HasSuffix(a.Name.Space, "/relationships") {
			return a.Value
		}
	}
	return ""
}
func (n *officeNode) child(name string) *officeNode {
	for _, c := range n.Children {
		if c.Name.Local == name {
			return c
		}
	}
	return nil
}
func (n *officeNode) descendants(name string) []*officeNode {
	var result []*officeNode
	var walk func(*officeNode)
	walk = func(x *officeNode) {
		if x.Name.Local == name {
			result = append(result, x)
		}
		for _, c := range x.Children {
			walk(c)
		}
	}
	walk(n)
	return result
}

type officeRelationship struct {
	Type, Target string
	External     bool
}

func (p *officePackage) relationships(part string) (map[string]officeRelationship, error) {
	name := path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
	if part == "" {
		name = "_rels/.rels"
	}
	result := map[string]officeRelationship{}
	if p.files[name] == nil {
		return result, nil
	}
	n, err := p.node(name)
	if err != nil {
		return nil, err
	}
	for _, r := range n.Children {
		if r.Name.Local != "Relationship" {
			continue
		}
		id, target := r.attr("Id"), r.attr("Target")
		if id == "" || target == "" {
			return nil, fmt.Errorf("invalid relationship in %s", name)
		}
		if _, ok := result[id]; ok {
			return nil, fmt.Errorf("duplicate relationship %s in %s", id, name)
		}
		external := strings.EqualFold(r.attr("TargetMode"), "External")
		if !external {
			if strings.ContainsAny(target, "\\\x00?#") || strings.Contains(target, ":") {
				return nil, fmt.Errorf("unsafe relationship target %q", target)
			}
			if strings.HasPrefix(target, "/") {
				target = path.Clean(strings.TrimPrefix(target, "/"))
			} else {
				target = path.Clean(path.Join(path.Dir(part), target))
			}
			if target == "." || target == ".." || strings.HasPrefix(target, "../") {
				return nil, fmt.Errorf("escaping relationship target")
			}
		}
		result[id] = officeRelationship{Type: r.attr("Type"), Target: target, External: external}
	}
	return result, nil
}

func (p *officePackage) presentation() (string, *officeNode, error) {
	rels, err := p.relationships("")
	if err != nil {
		return "", nil, err
	}
	for _, r := range rels {
		if strings.HasSuffix(r.Type, "/officeDocument") && !r.External {
			n, e := p.node(r.Target)
			if e != nil {
				return "", nil, e
			}
			if n.Name.Local != "presentation" {
				return "", nil, fmt.Errorf("Office input is not a presentation")
			}
			return r.Target, n, nil
		}
	}
	return "", nil, fmt.Errorf("Office archive has no presentation relationship")
}

// Only self-contained theme XML is reused. Relationships, masters, layouts and
// executable/embedded content from the source template are never copied.
func officeTemplateTheme(filename string) (string, error) {
	p, err := openOfficePackage(filename)
	if err != nil {
		return "", err
	}
	defer p.archive.Close()
	part, n, err := p.presentation()
	if err != nil {
		return "", err
	}
	rels, err := p.relationships(part)
	if err != nil {
		return "", err
	}
	masters := n.descendants("sldMasterId")
	if len(masters) == 0 {
		return "", fmt.Errorf("template has no slide master")
	}
	master, ok := rels[masters[0].relID()]
	if !ok || master.External || !strings.HasSuffix(master.Type, "/slideMaster") {
		return "", fmt.Errorf("template master relationship is invalid")
	}
	mrels, err := p.relationships(master.Target)
	if err != nil {
		return "", err
	}
	for _, r := range mrels {
		if strings.HasSuffix(r.Type, "/theme") && !r.External {
			data, e := p.read(r.Target, officeMaxXML)
			if e != nil {
				return "", e
			}
			node, e := parseOfficeXML(data)
			if e != nil {
				return "", e
			}
			if node.Name.Local != "theme" || node.Name.Space != "http://schemas.openxmlformats.org/drawingml/2006/main" || node.child("themeElements") == nil {
				return "", fmt.Errorf("invalid template theme")
			}
			var validate func(*officeNode) error
			validate = func(n *officeNode) error {
				for _, a := range n.Attrs {
					if strings.HasSuffix(a.Name.Space, "/relationships") {
						return fmt.Errorf("template theme requires external parts")
					}
				}
				for _, c := range n.Children {
					if e := validate(c); e != nil {
						return e
					}
				}
				return nil
			}
			if e = validate(node); e != nil {
				return "", e
			}
			body := strings.TrimSpace(string(data))
			if strings.HasPrefix(body, "<?xml") {
				if end := strings.Index(body, "?>"); end >= 0 {
					body = strings.TrimSpace(body[end+2:])
				}
			}
			return body, nil
		}
	}
	return "", fmt.Errorf("template master has no theme relationship")
}
