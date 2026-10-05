package slides

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"m31labs.dev/gosx/server"
)

type exportAsset struct {
	source, destination string
}

func managedIslandAsset(relative string) bool {
	relative = strings.ToLower(filepath.ToSlash(relative))
	return strings.HasPrefix(relative, "islands/") || strings.HasPrefix(relative, "assets/islands/")
}

// Reuse the installed GoSX public policy rather than duplicating its secret,
// state, path and symlink rules. HEAD reads no asset body into this recorder.
func exportPublicPolicy(directory string) func(string) bool {
	app := server.New()
	app.SetPublicDir(directory)
	handler := app.Build()
	return func(relative string) bool {
		target := (&url.URL{Path: "/" + filepath.ToSlash(relative)}).String()
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodHead, target, nil))
		return response.Code == http.StatusOK
	}
}

// Resolve existing parents too, so symlink aliases cannot bypass overlap
// checks for an output directory which has not been created yet.
func exportCanonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(abs)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", err
		}
		tail = append(tail, filepath.Base(abs))
		abs = parent
	}
}

func exportWithin(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func exportTreePlan(source, destination string, build bool) ([]exportAsset, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("export source %s must be a directory without symlinks", source)
	}
	sourceCanonical, err := exportCanonicalPath(source)
	if err != nil {
		return nil, err
	}
	destinationCanonical, err := exportCanonicalPath(destination)
	if err != nil {
		return nil, err
	}
	if exportWithin(sourceCanonical, destinationCanonical) || exportWithin(destinationCanonical, sourceCanonical) {
		return nil, fmt.Errorf("export source and destination overlap: %s and %s", source, destination)
	}
	allowed := exportPublicPolicy(source)
	var assets []exportAsset
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("export source contains symlink: %s", path)
		}
		if entry.IsDir() {
			if build && path != source && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("export source is not a regular file: %s", path)
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if (build && strings.HasPrefix(entry.Name(), ".")) || !allowed(relative) {
			return nil
		}
		if build && relative == "gosx-runtime.wasm" {
			relative = "runtime.wasm"
		}
		assets = append(assets, exportAsset{path, filepath.Join(destination, relative)})
		return nil
	})
	return assets, err
}

func planSPAAssets(directory string, deck *IslandDeck, output string, includeNotes bool) ([]exportAsset, error) {
	author, err := exportCanonicalPath(directory)
	if err != nil {
		return nil, err
	}
	destination, err := exportCanonicalPath(output)
	if err != nil {
		return nil, err
	}
	if exportWithin(destination, author) {
		return nil, fmt.Errorf("SPA output must not contain the authored deck directory")
	}
	assets, err := exportTreePlan(filepath.Join(directory, "build"), filepath.Join(output, "gosx"), true)
	if err != nil {
		return nil, fmt.Errorf("preflight runtime assets: %w", err)
	}
	// Slides stages only flat, named JSON programs for compiled components. Do
	// not publish unknown nested/hashed artifacts from a previous GoSX build.
	compiled, _ := deck.compileComponents()
	programs := make(map[string]bool, len(compiled))
	for name := range compiled {
		programs[filepath.Join("islands", name+".json")] = true
	}
	for _, asset := range assets {
		relative, err := filepath.Rel(filepath.Join(output, "gosx"), asset.destination)
		if err != nil {
			return nil, err
		}
		if managedIslandAsset(relative) && !programs[relative] {
			return nil, fmt.Errorf("unplanned island program in runtime assets: %s; use a fresh build/islands directory", asset.source)
		}
	}
	public := filepath.Join(directory, "public")
	if _, err := os.Lstat(public); err == nil {
		files, err := exportTreePlan(public, filepath.Join(output, "public"), false)
		if err != nil {
			return nil, fmt.Errorf("preflight public assets: %w", err)
		}
		assets = append(assets, files...)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	// Registered include and pack resources are explicit publications; refuse
	// private or nonregular resources rather than leaving a broken exported URL.
	allowed := exportPublicPolicy(directory)
	var routes []string
	for route := range deck.compositionAssets {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		relative := deck.compositionAssets[route]
		if !allowed(filepath.FromSlash(relative)) {
			return nil, fmt.Errorf("included/pack asset is denied by the public asset policy: %s", relative)
		}
		source, err := safeAuthorPath(directory, relative)
		if err != nil {
			return nil, err
		}
		assets = append(assets, exportAsset{source, filepath.Join(output, filepath.FromSlash(strings.TrimPrefix(route, "/")))})
	}
	planned := make(map[string]bool, len(assets))
	for _, asset := range assets {
		planned[filepath.Clean(asset.destination)] = true
	}
	// Previous bundles can otherwise retain a secret file or an island program
	// omitted from the new audience. Refuse unsafe output before changing HTML.
	for _, name := range []string{"public", "gosx"} {
		root := filepath.Join(output, name)
		if _, err := os.Lstat(root); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		allowed := exportPublicPolicy(root)
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 || (!entry.IsDir() && !entry.Type().IsRegular()) {
				return fmt.Errorf("existing export contains symlink or nonregular path: %s", path)
			}
			if !entry.IsDir() {
				relative, err := filepath.Rel(root, path)
				if err != nil || !allowed(relative) {
					return fmt.Errorf("existing export contains a file denied by public asset policy: %s; use a fresh output directory", path)
				}
				if name == "gosx" && managedIslandAsset(relative) && !planned[filepath.Clean(path)] {
					return fmt.Errorf("existing export contains an unplanned island program: %s; use a fresh output directory", path)
				}
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	for _, path := range []string{filepath.Join(output, "index.html"), filepath.Join(output, "notes.html")} {
		if filepath.Base(path) == "notes.html" && !includeNotes {
			continue
		}
		if err := exportTargetPreflight(output, path); err != nil {
			return nil, err
		}
	}
	for _, asset := range assets {
		if err := exportTargetPreflight(output, asset.destination); err != nil {
			return nil, err
		}
	}
	return assets, nil
}

func exportTargetPreflight(output, target string) error {
	root, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	path, err := filepath.Abs(target)
	if err != nil || !exportWithin(root, path) {
		return fmt.Errorf("export target escapes output directory: %s", target)
	}
	filePath := path
	for {
		info, err := os.Lstat(path)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 || (path == filePath && !info.Mode().IsRegular()) || (path != filePath && !info.IsDir()) {
				return fmt.Errorf("export target contains symlink or incompatible file: %s", path)
			}
		} else if !os.IsNotExist(err) {
			return err
		}
		if path == root {
			return nil
		}
		path = filepath.Dir(path)
	}
}

func copyExportAssets(assets []exportAsset) error {
	for _, asset := range assets {
		info, err := os.Lstat(asset.source)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("export source changed or is not regular: %s", asset.source)
		}
		if err := copyFile(asset.destination, asset.source); err != nil {
			return err
		}
	}
	return nil
}
