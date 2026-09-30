package slides

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// The cache follows the resolved dependency graph and the deck's actual Go
// toolchain, including automatic toolchain selection for portable deck modules.
func runtimeCacheKey(root, deckDir string) (string, error) {
	metadata, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Path}} {{.Version}}{{if .Replace}} {{.Replace.Dir}}{{end}}", "all")
	cmd.Dir = deckDir
	cmd.Env = append(execEnvWithoutGoFlags(), "GOWORK=off", "GOFLAGS=-mod=mod")
	graph, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve runtime dependency graph: %w", err)
	}
	toolchain, err := deckGoToolchain(deckDir)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x\n", sha256.Sum256([]byte(root+"\n"+toolchain.Root+"\n"+toolchain.Version+"\n"+string(metadata)+"\n"+string(graph)))), nil
}
func deckGoRoot(deckDir string) (string, error) {
	toolchain, err := deckGoToolchain(deckDir)
	return toolchain.Root, err
}

type goToolchain struct {
	Root    string `json:"GOROOT"`
	Version string `json:"GOVERSION"`
}

func deckGoToolchain(deckDir string) (goToolchain, error) {
	cmd := exec.Command("go", "env", "-json", "GOROOT", "GOVERSION")
	cmd.Dir = deckDir
	cmd.Env = append(execEnvWithoutGoFlags(), "GOWORK=off", "GOFLAGS=-mod=mod")
	data, err := cmd.Output()
	if err != nil {
		return goToolchain{}, fmt.Errorf("resolve deck Go toolchain: %w", err)
	}
	var toolchain goToolchain
	if err := json.Unmarshal(data, &toolchain); err != nil {
		return goToolchain{}, fmt.Errorf("decode deck Go toolchain: %w", err)
	}
	if toolchain.Root == "" || toolchain.Version == "" {
		return goToolchain{}, fmt.Errorf("deck Go toolchain omitted GOROOT or GOVERSION")
	}
	return toolchain, nil
}
