package slides

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
)

const runtimeBundleManifest = "bundle.json"
const maxRuntimeBundleBytes = 96 << 20

// RuntimeBundle identifies a release's portable client runtime. Assets are
// integrity checked before any deck files are changed. No downloads are made.
type RuntimeBundle struct {
	Version     int                           `json:"version"`
	GoSXVersion string                        `json:"gosxVersion"`
	GoVersion   string                        `json:"goVersion"`
	Assets      map[string]RuntimeBundleAsset `json:"assets"`
}

type RuntimeBundleAsset struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type loadedRuntimeBundle struct {
	manifest RuntimeBundle
	files    map[string][]byte
}

func runtimeBundleAssetName(name string) bool {
	return name == "gosx-runtime.wasm" || name == "wasm_exec.js" || name == "patch.js" ||
		(strings.HasPrefix(name, "bootstrap") && strings.HasSuffix(name, ".js") && filepath.Base(name) == name && !strings.ContainsAny(name, `/\\`))
}

func runtimeBundlePath() (string, bool, error) {
	if explicit := os.Getenv("SLIDES_RUNTIME_DIR"); explicit != "" {
		return explicit, true, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", false, err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	dir := filepath.Join(filepath.Dir(executable), "runtime")
	if _, err := os.Stat(filepath.Join(dir, runtimeBundleManifest)); os.IsNotExist(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}
	return dir, true, nil
}

func binaryGoSXReplaced() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, dep := range info.Deps {
		if dep.Path == gosxModuleImportPath && dep.Replace != nil {
			return true
		}
	}
	return false
}

func readRuntimeBundle(dir string) (*loadedRuntimeBundle, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open bundled runtime: %w", err)
	}
	defer root.Close()
	read := func(name string, limit int64) ([]byte, error) {
		info, err := root.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
			return nil, fmt.Errorf("runtime asset %s must be a regular file below %d bytes", name, limit)
		}
		file, err := root.Open(name)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, limit+1))
		if len(data) > int(limit) {
			return nil, fmt.Errorf("runtime asset %s exceeds its size budget", name)
		}
		return data, err
	}
	data, err := read(runtimeBundleManifest, 128<<10)
	if err != nil {
		return nil, err
	}
	var manifest RuntimeBundle
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("decode bundled runtime: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("bundled runtime needs one manifest")
	}
	if manifest.Version != 1 || manifest.GoSXVersion != gosxScaffoldVersion() || manifest.GoVersion == "" || binaryGoSXReplaced() {
		return nil, fmt.Errorf("bundled runtime does not match this binary's GoSX %s; use --rebuild with Go for custom runtimes", gosxScaffoldVersion())
	}
	if len(manifest.Assets) < 6 || len(manifest.Assets) > 64 {
		return nil, fmt.Errorf("bundled runtime needs 6–64 assets")
	}
	for _, name := range []string{"gosx-runtime.wasm", "wasm_exec.js", "patch.js", "bootstrap.js", "bootstrap-lite.js", "bootstrap-runtime.js"} {
		if _, ok := manifest.Assets[name]; !ok {
			return nil, fmt.Errorf("bundled runtime omits %s", name)
		}
	}
	loaded := &loadedRuntimeBundle{manifest: manifest, files: make(map[string][]byte)}
	var total int64
	for name, asset := range manifest.Assets {
		limit := int64(4 << 20)
		if name == "gosx-runtime.wasm" {
			limit = 64 << 20
		}
		if !runtimeBundleAssetName(name) || asset.Size <= 0 || asset.Size > limit || len(asset.SHA256) != 64 {
			return nil, fmt.Errorf("invalid bundled runtime asset %q", name)
		}
		total += asset.Size
		if total > maxRuntimeBundleBytes {
			return nil, fmt.Errorf("bundled runtime exceeds 96 MiB")
		}
		content, err := read(name, asset.Size)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(content)
		if int64(len(content)) != asset.Size || hex.EncodeToString(sum[:]) != asset.SHA256 {
			return nil, fmt.Errorf("bundled runtime integrity failed for %s", name)
		}
		if name == "gosx-runtime.wasm" && (len(content) < 1<<20 || string(content[:8]) != "\x00asm\x01\x00\x00\x00") {
			return nil, fmt.Errorf("bundled runtime has an invalid WASM artifact")
		}
		loaded.files[name] = content
	}
	return loaded, nil
}

func stageBundledRuntime(deckDir string, needsWASM bool) (bool, error) {
	dir, available, err := runtimeBundlePath()
	if err != nil || !available {
		return false, err
	}
	bundle, err := readRuntimeBundle(dir)
	if err != nil {
		return false, err
	}
	root, err := os.OpenRoot(deckDir)
	if err != nil {
		return false, err
	}
	defer root.Close()
	if err := root.Mkdir("build", 0755); err != nil && !os.IsExist(err) {
		return false, err
	}
	info, err := root.Lstat("build")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("bundled runtime needs a real build directory")
	}
	var names []string
	for name := range bundle.files {
		if name == "gosx-runtime.wasm" && !needsWASM {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	// Preflight the entire destination before copying. Temporary siblings and
	// rename never follow an existing file symlink or publish partial file bytes.
	for _, name := range names {
		info, err := root.Lstat(filepath.Join("build", name))
		if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		if err == nil && !info.Mode().IsRegular() {
			return false, fmt.Errorf("runtime destination %s is not a regular file", name)
		}
	}
	build := filepath.Join(deckDir, "build")
	buildRoot, err := root.OpenRoot("build")
	if err != nil {
		return false, err
	}
	defer buildRoot.Close()
	for _, name := range names {
		if err := writeBundledRuntimeAsset(buildRoot, name, bundle.files[name]); err != nil {
			return false, err
		}
	}
	if err := stageRuntimeManifest(deckDir, build); err != nil {
		return false, err
	}
	return true, nil
}

func writeBundledRuntimeAsset(root *os.Root, name string, content []byte) error {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temporary := ".slides-runtime-" + hex.EncodeToString(random[:])
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer root.Remove(temporary)
	if _, err := file.Write(content); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return root.Rename(temporary, name)
}

// PackageRuntime builds a version-matched client bundle in a fresh directory.
// Release archives keep this runtime/ beside the CLI; ordinary serve/build then
// need neither Go nor network access. --rebuild and watch remain developer paths.
func PackageRuntime(deckDir, destination string) (RuntimeBundle, error) {
	var manifest RuntimeBundle
	abs, err := filepath.Abs(destination)
	if err != nil {
		return manifest, err
	}
	if _, err := os.Lstat(abs); err == nil || !os.IsNotExist(err) {
		return manifest, fmt.Errorf("runtime package needs a fresh destination")
	}
	cmd := exec.Command("go", "list", "-m", "-json", gosxModuleImportPath)
	cmd.Dir = deckDir
	cmd.Env = append(execEnvWithoutGoFlags(), "GOFLAGS=-mod=mod")
	metadata, err := cmd.Output()
	if err != nil {
		return manifest, fmt.Errorf("resolve release runtime: %w", err)
	}
	var module struct {
		Version string
		Main    bool
		Replace json.RawMessage
	}
	if err := json.Unmarshal(metadata, &module); err != nil || module.Version != gosxScaffoldVersion() || module.Main || len(module.Replace) != 0 || binaryGoSXReplaced() {
		return manifest, fmt.Errorf("release runtime needs the binary's unmodified GoSX %s", gosxScaffoldVersion())
	}
	root, err := stageRuntimeAssets(deckDir, true, true)
	if err != nil {
		return manifest, err
	}
	build := filepath.Join(root, "build")
	entries, err := os.ReadDir(build)
	if err != nil {
		return manifest, err
	}
	manifest = RuntimeBundle{Version: 1, GoSXVersion: gosxScaffoldVersion(), Assets: make(map[string]RuntimeBundleAsset)}
	toolchain, err := deckGoToolchain(deckDir)
	if err != nil {
		return manifest, err
	}
	manifest.GoVersion = toolchain.Version
	files := make(map[string][]byte)
	var total int
	for _, entry := range entries {
		name := entry.Name()
		if !runtimeBundleAssetName(name) {
			continue
		}
		info, err := os.Lstat(filepath.Join(build, name))
		if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<20 {
			return manifest, fmt.Errorf("cannot package runtime asset %s", name)
		}
		content, err := os.ReadFile(filepath.Join(build, name))
		if err != nil {
			return manifest, err
		}
		total += len(content)
		if total > maxRuntimeBundleBytes {
			return manifest, fmt.Errorf("runtime package exceeds 96 MiB")
		}
		sum := sha256.Sum256(content)
		manifest.Assets[name] = RuntimeBundleAsset{SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content))}
		files[name] = content
	}
	if err := os.Mkdir(abs, 0755); err != nil {
		return manifest, err
	}
	complete := false
	defer func() {
		if !complete {
			for name := range files {
				_ = os.Remove(filepath.Join(abs, name))
			}
			_ = os.Remove(filepath.Join(abs, runtimeBundleManifest))
			_ = os.Remove(abs)
		}
	}()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(abs, name), content, 0644); err != nil {
			return manifest, err
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return manifest, err
	}
	if err := os.WriteFile(filepath.Join(abs, runtimeBundleManifest), data, 0644); err != nil {
		return manifest, err
	}
	if _, err := readRuntimeBundle(abs); err != nil {
		return manifest, err
	}
	complete = true
	return manifest, nil
}
