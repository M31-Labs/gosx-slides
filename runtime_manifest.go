package slides

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"m31labs.dev/gosx/buildmanifest"
)

// stageRuntimeManifest publishes the actual staged bytes' integrity metadata.
// GoSX reads dist/build.json beneath the runtime root. File stays empty because
// slides uses the supported /gosx/* compatibility routes, rather than the
// production /gosx/assets/runtime/* layout. This preserves the dev proxy and
// SPA export paths while allowing the browser to verify WASM and script bytes.
func stageRuntimeManifest(deckDir, buildDir string) error {
	manifest := buildmanifest.Manifest{}
	runtime := &manifest.Runtime
	assets := map[string]*buildmanifest.HashedAsset{
		"gosx-runtime.wasm": &runtime.WASM, "wasm_exec.js": &runtime.WASMExec,
		"patch.js":                                     &runtime.Patch,
		"bootstrap.js":                                 &runtime.Bootstrap,
		"bootstrap-lite.js":                            &runtime.BootstrapLite,
		"bootstrap-runtime.js":                         &runtime.BootstrapRuntime,
		"bootstrap-feature-islands.js":                 &runtime.BootstrapFeatureIslands,
		"bootstrap-feature-engines.js":                 &runtime.BootstrapFeatureEngines,
		"bootstrap-feature-hubs.js":                    &runtime.BootstrapFeatureHubs,
		"bootstrap-feature-controllers.js":             &runtime.BootstrapFeatureControllers,
		"bootstrap-feature-textlayout.js":              &runtime.BootstrapFeatureTextlayout,
		"bootstrap-feature-scene3d.js":                 &runtime.BootstrapFeatureScene3D,
		"bootstrap-feature-scene3d-command.js":         &runtime.BootstrapFeatureScene3DCommand,
		"bootstrap-feature-scene3d-hydrate.js":         &runtime.BootstrapFeatureScene3DHydrate,
		"bootstrap-feature-scene3d-webgpu.js":          &runtime.BootstrapFeatureScene3DWebGPU,
		"bootstrap-feature-scene3d-webgl.js":           &runtime.BootstrapFeatureScene3DWebGL,
		"bootstrap-feature-scene3d-gltf.js":            &runtime.BootstrapFeatureScene3DGLTF,
		"bootstrap-feature-scene3d-animation.js":       &runtime.BootstrapFeatureScene3DAnimation,
		"bootstrap-feature-scene3d-compute.js":         &runtime.BootstrapFeatureScene3DCompute,
		"bootstrap-feature-scene3d-decompress.js":      &runtime.BootstrapFeatureScene3DDecompress,
		"bootstrap-feature-scene3d-instance-stream.js": &runtime.BootstrapFeatureScene3DInstanceStream,
	}
	for name, asset := range assets {
		data, err := os.ReadFile(filepath.Join(buildDir, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("hash %s: %w", name, err)
		}
		*asset = buildmanifest.HashedAsset{
			Hash: buildmanifest.ContentHash(data), Size: int64(len(data)),
			Integrity: buildmanifest.ContentIntegrity(data),
		}
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(deckDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Mkdir("dist", 0755); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := root.Lstat("dist")
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("runtime manifest needs a real dist directory")
	}
	dist, err := root.OpenRoot("dist")
	if err != nil {
		return err
	}
	defer dist.Close()
	if info, err := dist.Lstat("build.json"); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("runtime manifest destination must be a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return writeBundledRuntimeAsset(dist, "build.json", data)
}
