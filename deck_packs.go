package slides

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"m31labs.dev/gosx/server"
	"m31labs.dev/mdpp"
)

// PackManifest is the versioned, local authoring-pack contract. Packs contain
// CSS, registered layouts and explicitly named GoSX components. They execute
// no installation hooks and stay inside the deck's packs/<name> directory.
type PackManifest struct {
	Schema     int               `json:"schema"`
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	BaseTheme  string            `json:"baseTheme,omitempty"`
	CSS        []string          `json:"css,omitempty"`
	Layouts    []string          `json:"layouts,omitempty"`
	Components map[string]string `json:"components,omitempty"`
}

type DeckPack struct {
	PackManifest
	Path string `json:"path"`
	css  []string
}

var packNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
var packVersionRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)

// DeckPacks lists enabled packs in authored cascade order.
func DeckPacks(deck *IslandDeck) []DeckPack {
	if deck == nil {
		return nil
	}
	return append([]DeckPack(nil), deck.Packs...)
}

func readPackManifest(dir string) (PackManifest, error) {
	var manifest PackManifest
	path, err := safeAuthorPath(dir, "pack.json")
	if err != nil {
		return manifest, fmt.Errorf("pack manifest: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return manifest, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 65537))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return manifest, fmt.Errorf("invalid pack.json: %w", err)
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return manifest, fmt.Errorf("pack.json must contain one manifest")
	}
	if manifest.Schema != 1 || !packNameRe.MatchString(manifest.Name) || !packVersionRe.MatchString(manifest.Version) {
		return manifest, fmt.Errorf("pack.json requires schema 1, a lowercase name and an exact semantic version")
	}
	if manifest.BaseTheme != "" {
		if _, ok := themeRegistry[manifest.BaseTheme]; !ok {
			return manifest, fmt.Errorf("pack %s: unknown baseTheme %q", manifest.Name, manifest.BaseTheme)
		}
	}
	for _, layout := range manifest.Layouts {
		if !packNameRe.MatchString(layout) {
			return manifest, fmt.Errorf("pack %s: invalid layout %q", manifest.Name, layout)
		}
	}
	for name, path := range manifest.Components {
		if !sceneComponentNameRe.MatchString(name) || isGraphicsComponent(name) || name == reservedNotesTag {
			return manifest, fmt.Errorf("pack %s: invalid component %q", manifest.Name, name)
		}
		if !strings.EqualFold(filepath.Ext(path), ".gsx") {
			return manifest, fmt.Errorf("pack %s: component %s must name .gsx source", manifest.Name, name)
		}
		if err := regularPackFile(dir, path); err != nil {
			return manifest, fmt.Errorf("pack %s component %s: %w", manifest.Name, name, err)
		}
	}
	for _, path := range manifest.CSS {
		if !strings.EqualFold(filepath.Ext(path), ".css") {
			return manifest, fmt.Errorf("pack %s: stylesheet must name .css source", manifest.Name)
		}
		if err := regularPackFile(dir, path); err != nil {
			return manifest, fmt.Errorf("pack %s CSS: %w", manifest.Name, err)
		}
	}
	return manifest, nil
}

func regularPackFile(dir, rel string) error {
	path, err := safeAuthorPath(dir, rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("pack source is not a regular file: %s", rel)
	}
	return nil
}

func (d *IslandDeck) loadPacks() error {
	seen := map[string]bool{}
	for _, pin := range strings.FieldsFunc(deckFrontmatterString(d, "packs"), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		name, version, ok := strings.Cut(pin, "@")
		if !ok || !packNameRe.MatchString(name) || !packVersionRe.MatchString(version) {
			return fmt.Errorf("packs: use exact name@version pins, got %q", pin)
		}
		if seen[name] {
			return fmt.Errorf("packs: duplicate pack %q", name)
		}
		seen[name] = true
		rel := "packs/" + name
		path, err := safeAuthorPath(d.Dir, rel)
		if err != nil {
			return fmt.Errorf("pack %s: %w", pin, err)
		}
		manifest, err := readPackManifest(path)
		if err != nil {
			return fmt.Errorf("pack %s: %w", pin, err)
		}
		if manifest.Name != name || manifest.Version != version {
			return fmt.Errorf("pack %s: installed manifest is %s@%s", pin, manifest.Name, manifest.Version)
		}
		pack := DeckPack{PackManifest: manifest, Path: rel}
		for _, css := range manifest.CSS {
			data, err := os.ReadFile(filepath.Join(path, filepath.FromSlash(css)))
			if err != nil {
				return err
			}
			rewritten, err := d.rebasePackCSS(rel+"/"+css, string(data))
			if err != nil {
				return fmt.Errorf("pack %s: %w", pin, err)
			}
			pack.css = append(pack.css, rewritten)
		}
		for component, file := range manifest.Components {
			if _, err := safeAuthorPath(d.Dir, component+".gsx"); err == nil {
				continue
			}
			relSource := filepath.ToSlash(filepath.Join(rel, file))
			if current := d.componentSources[component]; current != "" && current != relSource {
				return fmt.Errorf("packs: component %s is provided by both %s and %s", component, current, relSource)
			}
			d.componentSources[component] = relSource
		}
		for _, layout := range manifest.Layouts {
			d.packLayouts[layout] = true
		}
		d.Packs = append(d.Packs, pack)
	}
	return nil
}

var cssAssetURL = regexp.MustCompile(`(?i)url\(\s*("[^"]*"|'[^']*'|[^\s)]+)\s*\)`)

// CSS assets are relative to their stylesheet, including fonts. An @import
// hides additional dependency/cascade behavior; make pack CSS self-contained.
func (d *IslandDeck) rebasePackCSS(origin, css string) (string, error) {
	if strings.Contains(strings.ToLower(css), "@import") {
		return "", fmt.Errorf("%s: pack CSS must be self-contained; @import is unsupported", origin)
	}
	var problem error
	css = cssAssetURL.ReplaceAllStringFunc(css, func(ref string) string {
		match := cssAssetURL.FindStringSubmatch(ref)
		raw := strings.Trim(match[1], "\"'")
		if strings.HasPrefix(raw, "#") {
			return ref
		}
		value, err := d.assetURL(origin, raw)
		if err != nil {
			problem = err
			return ref
		}
		return `url("` + value + `")`
	})
	return css, problem
}

// InstallDeckPack vendors a reviewed local directory. It validates before
// publishing and refuses replacement; update by installing into a new deck or
// removing the old pack explicitly. No network fetch or executable hooks occur.
func InstallDeckPack(deckDir, sourceDir string) (PackManifest, error) {
	manifest, err := readPackManifest(sourceDir)
	if err != nil {
		return manifest, err
	}
	validation := &IslandDeck{Dir: sourceDir}
	for _, css := range manifest.CSS {
		data, err := os.ReadFile(filepath.Join(sourceDir, filepath.FromSlash(css)))
		if err != nil {
			return manifest, err
		}
		if _, err := validation.rebasePackCSS(css, string(data)); err != nil {
			return manifest, err
		}
	}
	destination := filepath.Join(deckDir, "packs", manifest.Name)
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return manifest, fmt.Errorf("pack destination already exists: %s", destination)
	}
	root, err := filepath.EvalSymlinks(sourceDir)
	if err != nil {
		return manifest, err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return manifest, err
	}
	deckRoot, err := filepath.Abs(deckDir)
	if err != nil {
		return manifest, err
	}
	deckRoot, err = filepath.EvalSymlinks(deckRoot)
	if err != nil {
		return manifest, err
	}
	if nested, err := filepath.Rel(root, filepath.Join(deckRoot, "packs", manifest.Name)); err == nil && safeDeckRelPath(nested) {
		return manifest, fmt.Errorf("pack install destination must be outside its source directory")
	}
	deckDirectory, err := os.OpenRoot(deckRoot)
	if err != nil {
		return manifest, err
	}
	defer deckDirectory.Close()
	if info, err := deckDirectory.Lstat("packs"); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return manifest, fmt.Errorf("pack installation rejects a symlinked packs directory")
	}
	if err := deckDirectory.MkdirAll("packs", 0755); err != nil {
		return manifest, err
	}
	packsDirectory, err := deckDirectory.OpenRoot("packs")
	if err != nil {
		return manifest, err
	}
	defer packsDirectory.Close()
	lockName := ".install-" + manifest.Name + ".lock"
	lock, err := packsDirectory.OpenFile(lockName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return manifest, fmt.Errorf("pack installation is already in progress: %w", err)
	}
	lock.Close()
	defer packsDirectory.Remove(lockName)
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return manifest, err
	}
	stagedName := fmt.Sprintf(".install-%x", nonce)
	if err := packsDirectory.Mkdir(stagedName, 0755); err != nil {
		return manifest, err
	}
	defer packsDirectory.RemoveAll(stagedName)
	stagedDirectory, err := packsDirectory.OpenRoot(stagedName)
	if err != nil {
		return manifest, err
	}
	defer stagedDirectory.Close()
	sourceDirectory, err := os.OpenRoot(root)
	if err != nil {
		return manifest, err
	}
	defer sourceDirectory.Close()
	var total int64
	files := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("pack install rejects symlink %s", rel)
		}
		if entry.IsDir() {
			return stagedDirectory.MkdirAll(rel, 0755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("pack install rejects non-regular file %s", rel)
		}
		files++
		file, err := sourceDirectory.Open(rel)
		if err != nil {
			return err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("pack source is not a regular file: %s", rel)
		}
		if info.Size() > snapshotAssetLimit || files > 4096 {
			return fmt.Errorf("pack installation exceeds file/size budget (16 MiB each, 96 MiB total, 4096 files)")
		}
		data, err := io.ReadAll(io.LimitReader(file, snapshotAssetLimit+1))
		if err != nil {
			return err
		}
		if len(data) > snapshotAssetLimit {
			return fmt.Errorf("pack source grew beyond size budget")
		}
		total += int64(len(data))
		if total > snapshotTotalLimit {
			return fmt.Errorf("pack installation exceeds the 96 MiB size budget")
		}
		return stagedDirectory.WriteFile(rel, data, 0644)
	})
	if err != nil {
		return manifest, err
	}
	if _, err := readPackManifest(filepath.Join(deckRoot, "packs", stagedName)); err != nil {
		return manifest, err
	}
	if _, err := packsDirectory.Lstat(manifest.Name); !os.IsNotExist(err) {
		return manifest, fmt.Errorf("pack destination already exists: %s", destination)
	}
	if err := packsDirectory.Rename(stagedName, manifest.Name); err != nil {
		return manifest, err
	}
	return manifest, nil
}

// resolveDeckComponents chooses explicit deck files first, then fragment-local
// definitions, then pack definitions. Conflicting fragment definitions fail
// instead of silently compiling a different island under the same name.
func (d *IslandDeck) resolveDeckComponents() error {
	var problem error
	localSources := map[string]string{}
	bind := func(name, origin string) {
		if name == "" || isGraphicsComponent(name) || name == reservedNotesTag {
			return
		}
		if _, err := safeAuthorPath(d.Dir, name+".gsx"); err == nil {
			return
		}
		path := filepath.ToSlash(filepath.Join(filepath.Dir(origin), name+".gsx"))
		if _, err := safeAuthorPath(d.Dir, path); os.IsNotExist(err) {
			return
		} else if err != nil {
			problem = err
			return
		}
		if current := localSources[name]; current != "" && current != path {
			problem = fmt.Errorf("component %s conflicts between %s and %s; provide a deck-level override", name, current, path)
			return
		}
		localSources[name] = path
		d.componentSources[name] = path
	}
	d.Document.AST().Walk(func(n *mdpp.Node) bool {
		if problem != nil {
			return false
		}
		where, ok := d.SourceLocation(n.Range.StartByte, n.Range.StartByte+1)
		if !ok || where.File == DeckFileName {
			return true
		}
		var refs []ComponentRef
		if n.Type == mdpp.NodeComponent {
			refs = []ComponentRef{{Name: n.Attr("name")}}
		}
		if n.Type == mdpp.NodeHTMLBlock || n.Type == mdpp.NodeHTMLInline || n.Type == mdpp.NodeText {
			refs = scanLiteralComponents(n.Literal)
		}
		for _, ref := range refs {
			bind(ref.Name, where.File)
		}
		return true
	})
	if problem != nil {
		return problem
	}
	for _, slide := range d.Slides {
		if scene := parseFrontmatter(slide.Node.Attr("frontmatter"))["scene"]; scene != "" && len(slide.Node.Children) > 0 {
			start := slide.Node.Children[0].Range.StartByte
			if where, ok := d.SourceLocation(start, start+1); ok && where.File != DeckFileName {
				bind(sceneComponentName(scene), where.File)
			}
		}
		for _, ref := range slide.Components {
			if isGraphicsComponent(ref.Name) {
				continue
			}
			rel := ref.Name + ".gsx"
			if _, err := safeAuthorPath(d.Dir, rel); err == nil {
				d.componentSources[ref.Name] = rel
			} else if !os.IsNotExist(err) {
				return err
			}
		}
	}
	return problem
}

func mountCompositionAssets(app *server.App, deck *IslandDeck, fresh bool) {
	app.Mount("/public/_slides/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := deck
		if fresh {
			var err error
			current, err = LoadIslandDeck(deck.Dir)
			if err != nil {
				http.NotFound(w, r)
				return
			}
		}
		// Only registered assets are served; author source paths stay private.
		path, ok := resolveCompositionAsset(current, r.URL.Path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
	}))
}

func copyCompositionAssets(deck *IslandDeck, out string) error {
	var routes []string
	for route := range deck.compositionAssets {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		src, err := safeAuthorPath(deck.Dir, deck.compositionAssets[route])
		if err != nil {
			return err
		}
		if err := copyFile(filepath.Join(out, filepath.FromSlash(strings.TrimPrefix(route, "/"))), src); err != nil {
			return err
		}
	}
	return nil
}

func resolveCompositionAsset(deck *IslandDeck, value string) (string, bool) {
	if deck == nil {
		return "", false
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", false
	}
	rel, ok := deck.compositionAssets[u.Path]
	if !ok {
		return "", false
	}
	abs, err := safeAuthorPath(deck.Dir, rel)
	return abs, err == nil
}
