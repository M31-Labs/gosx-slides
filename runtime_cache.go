package slides

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	toolchain, err := deckGoRoot(deckDir)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x\n", sha256.Sum256([]byte(root+"\n"+toolchain+"\n"+string(metadata)+"\n"+string(graph)))), nil
}
func deckGoRoot(deckDir string) (string, error) {
	cmd := exec.Command("go", "env", "GOROOT")
	cmd.Dir = deckDir
	cmd.Env = append(execEnvWithoutGoFlags(), "GOWORK=off", "GOFLAGS=-mod=mod")
	root, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolve deck Go toolchain: %w", err)
	}
	return strings.TrimSpace(string(root)), nil
}
