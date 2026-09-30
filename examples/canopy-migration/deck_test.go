package main

import (
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/gosx"
)

// Exercise the example's own module graph, rather than the parent CLI's graph.
func TestDeckComponentsCompile(t *testing.T) {
	paths, err := filepath.Glob("*.gsx")
	if err != nil || len(paths) == 0 {
		t.Fatalf("component sources: %v", err)
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := gosx.Compile(source); err != nil {
				t.Fatalf("GoSX component: %v", err)
			}
		})
	}
}
