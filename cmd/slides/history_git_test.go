package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHistoryReadsCommittedSourceWithoutRunningTextconv(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("optional Git tool unavailable")
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("test Git command: %s %v", out, err)
		}
		return strings.TrimSpace(string(out))
	}
	git("init")
	file := filepath.Join(dir, "model.sir")
	first := "service api { sid: \"api\" }\n"
	if err := os.WriteFile(file, []byte(first), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "model.sir")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "first")
	commit := git("rev-parse", "HEAD")
	blob := git("rev-parse", "HEAD:model.sir")
	git("config", "diff.danger.textconv", "this-command-must-never-run")
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("*.sir diff=danger\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("PRIVATE uncommitted source\n"), 0644); err != nil {
		t.Fatal(err)
	}
	data, sha, err := readGitTourSnapshot(context.Background(), dir, "HEAD", "model.sir")
	if err != nil || string(data) != first || sha != commit {
		t.Fatalf("immutable snapshot: %q %q %v", data, sha, err)
	}
	t.Run("ignores inherited repository", func(t *testing.T) {
		t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "wrong-repository"))
		data, _, err := readGitTourSnapshot(context.Background(), dir, "HEAD", "model.sir")
		if err != nil || string(data) != first {
			t.Fatal("inherited environment changed repository", err)
		}
	})
	// Store a symlink as a tree entry without requiring host symlink privileges.
	git("update-index", "--add", "--cacheinfo", "120000,"+blob+",link.sir")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "link")
	if _, _, err := readGitTourSnapshot(context.Background(), dir, "HEAD", "link.sir"); err == nil {
		t.Fatal("committed symlink accepted as Sirena source")
	}
	for _, tc := range []struct{ ref, path string }{{"--help", "model.sir"}, {"HEAD", "../private.sir"}, {"HEAD", "other:model.sir"}, {"HEAD", "model.go"}} {
		if _, _, err := readGitTourSnapshot(context.Background(), dir, tc.ref, tc.path); err == nil {
			t.Fatal("invalid Git source accepted", tc)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := readGitTourSnapshot(ctx, dir, "HEAD", "model.sir"); err == nil {
		t.Fatal("cancelled Git tour continued")
	}
}

func TestGitHistoryNeverFetchesMissingPromisorObject(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("optional Git tool unavailable")
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		data, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture Git: %s %v", data, err)
		}
		return strings.TrimSpace(string(data))
	}
	git("init")
	if err := os.WriteFile(filepath.Join(dir, "model.sir"), []byte("service api\n"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "model.sir")
	git("-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "-m", "source")
	blob := git("rev-parse", "HEAD:model.sir")
	remote := filepath.Join(t.TempDir(), "remote.git")
	git("clone", "--bare", dir, remote)
	git("config", "remote.origin.url", remote)
	git("config", "remote.origin.promisor", "true")
	git("config", "extensions.partialClone", "origin")
	git("config", "protocol.file.allow", "always")
	object := filepath.Join(dir, ".git", "objects", blob[:2], blob[2:])
	if err := os.Remove(object); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readGitTourSnapshot(context.Background(), dir, "HEAD", "model.sir"); err == nil {
		t.Fatal("missing object fetched despite offline tour policy")
	}
	if _, err := os.Stat(object); !os.IsNotExist(err) {
		t.Fatal("Git wrote a fetched object", err)
	}
}

func TestTourOutputEnforcesBudget(t *testing.T) {
	output := &tourOutput{limit: 4}
	if _, err := output.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := output.Write([]byte("5")); err == nil || output.String() != "1234" {
		t.Fatal("output exceeded fixed budget")
	}
}
